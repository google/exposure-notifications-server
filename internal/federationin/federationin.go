// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package federationin handles pulling data from other federation servers.
package federationin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/federationin/database"
	"github.com/google/exposure-notifications-server/internal/federationin/model"
	"github.com/google/exposure-notifications-server/internal/pb/federation"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats"
	"go.opencensus.io/trace"
)

const (
	queryParam = "query-id"
)

var (
	fetchBatchSize = publishdb.InsertExposuresBatchSize

	ErrInvalidReportType = errors.New("invalid report type")
)

type (
	fetchFn               func(context.Context, *federation.FederationFetchRequest, ...grpc.CallOption) (*federation.FederationFetchResponse, error)
	insertExposuresFn     func(context.Context, *publishdb.InsertAndReviseExposuresRequest) (*publishdb.InsertAndReviseExposuresResponse, error)
	startFederationSyncFn func(context.Context, *model.FederationInQuery, time.Time) (int64, database.FinalizeSyncFn, error)
)

type pullDependencies struct {
	fetch               fetchFn
	insertExposures     insertExposuresFn
	startFederationSync startFederationSyncFn
}

// NewHandler returns a handler that will fetch server-to-server
// federation results for a single federation query.
func NewHandler(env *serverenv.ServerEnv, config *Config) http.Handler {
	return &handler{
		env:       env,
		db:        database.New(env.Database()),
		publishdb: publishdb.New(env.Database()),
		config:    config,
	}
}

type handler struct {
	env       *serverenv.ServerEnv
	db        *database.FederationInDB
	publishdb *publishdb.PublishDB
	config    *Config
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*federationin.handler).ServeHTTP")
	defer span.End()

	logger := logging.FromContext(ctx)

	queryIDs, ok := r.URL.Query()[queryParam]
	if !ok {
		stats.Record(ctx, mPullInvalidRequest.M(1))
		badRequestf(ctx, w, "%s is required", queryParam)
		return
	}
	if len(queryIDs) > 1 {
		stats.Record(ctx, mPullInvalidRequest.M(1))
		badRequestf(ctx, w, "only one %s allowed", queryParam)
		return
	}
	queryID := queryIDs[0]
	if queryID == "" {
		stats.Record(ctx, mPullInvalidRequest.M(1))
		badRequestf(ctx, w, "%s is required", queryParam)
		return
	}

	// Obtain lock to make sure there are no other processes working on this batch.
	lock := "query_" + queryID
	unlockFn, err := h.db.Lock(ctx, lock, h.config.Timeout)
	if err != nil {
		if errors.Is(err, coredb.ErrAlreadyLocked) {
			stats.Record(ctx, mPullLockContention.M(1))
			msg := fmt.Sprintf("Lock %s already in use. No work will be performed.", lock)
			logger.Infof(msg)
			fmt.Fprint(w, msg) // We return status 200 here so that Cloud Scheduler does not retry.
			return
		}
		internalErrorf(ctx, w, "Could not acquire lock %s for query %s: %v", lock, queryID, err)
		return
	}
	defer func() {
		if err := unlockFn(); err != nil {
			logger.Errorf("failed to unlock: %v", err)
		}
	}()

	query, err := h.db.GetFederationInQuery(ctx, queryID)
	if err != nil {
		if errors.Is(err, coredb.ErrNotFound) {
			badRequestf(ctx, w, "unknown %s", queryParam)
			return
		}
		internalErrorf(ctx, w, "Failed getting query %q: %v", queryID, err)
		return
	}

	cp, err := x509.SystemCertPool()
	if err != nil {
		internalErrorf(ctx, w, "Failed to access system cert pool: %v", err)
		return
	}

	if h.config.TLSCertFile != "" {
		b, err := ioutil.ReadFile(h.config.TLSCertFile)
		if err != nil {
			internalErrorf(ctx, w, "Failed to read cert file %q: %v", h.config.TLSCertFile, err)
			return
		}
		if !cp.AppendCertsFromPEM(b) {
			internalErrorf(ctx, w, "Failed to append credentials")
			return
		}
	}

	tlsConfig := &tls.Config{RootCAs: cp, InsecureSkipVerify: h.config.TLSSkipVerify}
	dialOpts := []grpc.DialOption{
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}

	var clientOpts []idtoken.ClientOption
	if h.config.CredentialsFile != "" {
		clientOpts = append(clientOpts, idtoken.WithCredentialsFile(h.config.CredentialsFile))
	}
	ts, err := idtoken.NewTokenSource(ctx, query.Audience, clientOpts...)
	if err != nil {
		internalErrorf(ctx, w, "Failed to create token source: %v", err)
		return
	}
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(oauth.TokenSource{
		TokenSource: ts,
	}))

	logger.Infof("Dialing %s", query.ServerAddr)
	conn, err := grpc.Dial(query.ServerAddr, dialOpts...)
	if err != nil {
		internalErrorf(ctx, w, "Failed to dial for query %q %s: %v", queryID, query.ServerAddr, err)
		return
	}
	defer conn.Close()
	client := federation.NewFederationClient(conn)

	timeoutContext, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	opts := pullOptions{
		deps: pullDependencies{
			fetch:               client.Fetch,
			insertExposures:     h.publishdb.InsertAndReviseExposures,
			startFederationSync: h.db.StartFederationInSync,
		},
		query:                        query,
		batchStart:                   time.Now(),
		truncateWindow:               h.config.TruncateWindow,
		maxIntervalStartAge:          h.config.MaxIntervalAge,
		maxMagnitudeSymptomOnsetDays: h.config.MaxMagnitudeSymptomOnsetDays,
		debugReleaseSameDay:          h.config.ReleaseSameDayKeys,
	}
	if err := pull(timeoutContext, &opts); err != nil {
		internalErrorf(ctx, w, "Federation query %q failed: %v", queryID, err)
		return
	}

	if errors.Is(timeoutContext.Err(), context.DeadlineExceeded) {
		logger.Infof("Federation puller timed out at %v before fetching entire set.", h.config.Timeout)
	}
}

type pullOptions struct {
	deps                         pullDependencies
	query                        *model.FederationInQuery
	batchStart                   time.Time
	truncateWindow               time.Duration
	maxIntervalStartAge          time.Duration
	maxMagnitudeSymptomOnsetDays uint
	debugReleaseSameDay          bool
	config                       *Config
}

// updateTimestamps takes the current known max[revised] timestamps and compares them
// to the state in the fetch response. If the state in the fetch response has a newer time,
// then the known max(es) are adjusted forward.
func updateTimestamps(max, maxRevised time.Time, state *federation.FetchState) (time.Time, time.Time) {
	if state == nil {
		// Didn't get any response, don't advance timestamps.
		return max, maxRevised
	}

	if state.KeyCursor != nil {
		if timestamp := time.Unix(state.KeyCursor.Timestamp, 0).UTC(); timestamp.After(max) {
			max = timestamp
		}
	}
	if state.RevisedKeyCursor != nil {
		if revisedTimestamp := time.Unix(state.RevisedKeyCursor.Timestamp, 0).UTC(); revisedTimestamp.After(maxRevised) {
			maxRevised = revisedTimestamp
		}
	}
	return max, maxRevised
}

// converts a federation ExposureKey to a model Exposure.
func buildExposure(e *federation.ExposureKey, config *Config) (*publishmodel.Exposure, error) {
	upperRegions := make([]string, len(e.Regions))
	for i, r := range e.Regions {
		upperRegions[i] = strings.ToUpper(strings.TrimSpace(r))
	}
	sort.Strings(upperRegions)

	exposure := publishmodel.Exposure{
		ExposureKey:      e.ExposureKey,
		TransmissionRisk: int(e.TransmissionRisk),
		Regions:          upperRegions,
		Traveler:         e.Traveler,
		IntervalNumber:   e.IntervalNumber,
		IntervalCount:    e.IntervalCount,
		LocalProvenance:  false,
	}
	switch e.ReportType {
	case federation.ExposureKey_CONFIRMED_TEST:
		exposure.ReportType = verifyapi.ReportTypeConfirmed
	case federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS:
		exposure.ReportType = verifyapi.ReportTypeClinical
	case federation.ExposureKey_REVOKED:
		exposure.ReportType = verifyapi.ReportTypeNegative
	case federation.ExposureKey_SELF_REPORT:
		if config.AcceptSelfReport {
			exposure.ReportType = verifyapi.ReportTypeClinical
		} else {
			return nil, ErrInvalidReportType
		}
	case federation.ExposureKey_RECURSIVE:
		if config.AcceptRecursive {
			exposure.ReportType = verifyapi.ReportTypeClinical
		} else {
			return nil, ErrInvalidReportType
		}
	default:
		return nil, ErrInvalidReportType
	}
	// Maybe backfill transmission risk
	exposure.TransmissionRisk = publishmodel.ReportTypeTransmissionRisk(exposure.ReportType, exposure.TransmissionRisk)

	if e.HasSymptomOnset {
		if ds := e.DaysSinceOnsetOfSymptoms; ds >= -1*int32(config.MaxMagnitudeSymptomOnsetDays) && ds <= int32(config.MaxMagnitudeSymptomOnsetDays) {
			exposure.SetDaysSinceSymptomOnset(ds)
		}
	}

	return &exposure, nil
}

func pull(ctx context.Context, opts *pullOptions) (err error) {
	ctx, span := trace.StartSpan(ctx, "federationin.pull")
	defer func() {
		if err != nil {
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: err.Error()})
		}
		span.End()
	}()

	logger := logging.FromContext(ctx)
	logger.Infof("Processing query %q", opts.query.QueryID)

	request := &federation.FederationFetchRequest{
		IncludeRegions:      opts.query.IncludeRegions,
		ExcludeRegions:      opts.query.ExcludeRegions,
		OnlyTravelers:       opts.query.OnlyTravelers,
		OnlyLocalProvenance: opts.query.OnlyLocalProvenance,
		MaxExposureKeys:     uint32(fetchBatchSize),
		State:               opts.query.FetchState(),
	}

	syncID, finalizeFn, err := opts.deps.startFederationSync(ctx, opts.query, opts.batchStart)
	if err != nil {
		return fmt.Errorf("starting federation sync for query %s: %w", opts.query.QueryID, err)
	}

	var maxTimestamp, maxRevisedTimestamp time.Time
	total := 0
	defer func() {
		logger.Infof("Inserted %d keys", total)
	}()

	createdAt := publishmodel.TruncateWindow(opts.batchStart, opts.truncateWindow)
	// Create the transform / validation settings
	transformSettings := publishmodel.KeyTransform{
		// An exposure key must have an interval >= minInterval (max configured age)
		MinStartInterval: publishmodel.IntervalNumber(opts.batchStart.Add(-1 * opts.maxIntervalStartAge)),
		// A key must have been issued on the device in the current interval or earlier.
		MaxStartInterval: publishmodel.IntervalNumber(opts.batchStart),
		// And the max valid interval is the maxStartInterval + 144
		MaxEndInteral:         publishmodel.IntervalNumber(opts.batchStart) + verifyapi.MaxIntervalCount,
		CreatedAt:             createdAt,
		ReleaseStillValidKeys: opts.debugReleaseSameDay,
		BatchWindow:           opts.truncateWindow,
	}

	partial := true
	nPartials := int64(0)
	for partial {
		nPartials++
		span.AddAttributes(trace.Int64Attribute("n_partial", nPartials))

		// TODO(mikehelmick): react to the context timeout and complete a chunk of work so next invocation can pick up where left off.

		response, err := opts.deps.fetch(ctx, request)
		if err != nil {
			return fmt.Errorf("fetching query %s: %w", opts.query.QueryID, err)
		}

		// Advance timestamps based on cursors.
		maxTimestamp, maxRevisedTimestamp = updateTimestamps(maxTimestamp, maxRevisedTimestamp, response.NextFetchState)

		if len(response.Keys) > 0 {
			// Build state for new inserts.
			newExposures := make([]*publishmodel.Exposure, 0, len(response.Keys))
			for _, key := range response.Keys {
				exposure, err := buildExposure(key, opts.config)
				if err != nil {
					logger.Debugw("invalid key on federation, skipping", "error", err)
					continue
				}
				// Fill in federation specific items.
				exposure.FederationSyncID = syncID
				exposure.FederationQueryID = opts.query.QueryID

				if err := exposure.AdjustAndValidate(&transformSettings); err != nil {
					logger.Debugw("invalid key on federation, skipping", "error", err)
					continue
				}

				newExposures = append(newExposures, exposure)
			}
			resp, err := opts.deps.insertExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
				Incoming:     newExposures,
				SkipRevions:  true,
				RequireToken: false,
			})
			if err != nil {
				return fmt.Errorf("inserting %d exposures: %w", len(newExposures), err)
			}
			// Success, update metrics
			stats.Record(ctx, mPullInserts.M(int64(resp.Inserted)))
			stats.Record(ctx, mPullDropped.M(int64(resp.Dropped)))

			total += int(resp.Inserted)
		} else {
			logger.Info("no primary keys in response")
		}

		// Handle any revised keys.
		if len(response.RevisedKeys) > 0 {
			// Build state for new inserts.
			revisedExposures := make([]*publishmodel.Exposure, 0, len(response.Keys))
			for _, key := range response.RevisedKeys {
				exposure, err := buildExposure(key, opts.config)
				if err != nil {
					return ErrInvalidReportType
				}
				// Fill in federation specific items.
				exposure.FederationSyncID = syncID
				exposure.FederationQueryID = opts.query.QueryID

				if err := exposure.AdjustAndValidate(&transformSettings); err != nil {
					logger.Errorw("invalid key on federation, skipping", "error", err)
					continue
				}

				revisedExposures = append(revisedExposures, exposure)
			}
			resp, err := opts.deps.insertExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
				Incoming:       revisedExposures,
				OnlyRevisions:  true,
				RequireToken:   false,
				RequireQueryID: true,
			})
			if err != nil {
				return fmt.Errorf("revising %d exposures: %w", len(revisedExposures), err)
			}
			// Success, update metrics
			stats.Record(ctx, mPullRevisions.M(int64(resp.Revised)))
			stats.Record(ctx, mPullDropped.M(int64(resp.Dropped)))

			total += int(resp.Revised)
		} else {
			logger.Info("no revised keys in response")
		}

		partial = response.PartialResponse
		request.State = response.NextFetchState
	}

	if err := finalizeFn(request.State, opts.query, total); err != nil {
		// TODO(mikehelmick): how do we clean up here? Just leave the records in and have the exporter eliminate them? Other?
		return fmt.Errorf("finalizing federation sync for query %s: %w", opts.query.QueryID, err)
	}

	return nil
}

func badRequestf(ctx context.Context, w http.ResponseWriter, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logging.FromContext(ctx).Debug(msg)
	http.Error(w, msg, http.StatusBadRequest)
}

func internalErrorf(ctx context.Context, w http.ResponseWriter, format string, args ...interface{}) {
	logging.FromContext(ctx).Errorf(format, args...)
	http.Error(w, "Internal error", http.StatusInternalServerError)
}
