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
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"

	"google.golang.org/api/idtoken"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	queryParam = "query-id"
)

var (
	fetchBatchSize = database.InsertExposuresBatchSize
)

type (
	fetchFn               func(context.Context, *pb.FederationFetchRequest, ...grpc.CallOption) (*pb.FederationFetchResponse, error)
	insertExposuresFn     func(context.Context, []*model.Exposure) error
	startFederationSyncFn func(context.Context, *coredb.FederationInQuery, time.Time) (int64, coredb.FinalizeSyncFn, error)
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
		db:        env.Database(),
		publishdb: database.New(env.Database()),
		config:    config,
	}
}

type handler struct {
	env       *serverenv.ServerEnv
	db        *coredb.DB
	publishdb *database.PublishDB
	config    *Config
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	metrics := h.env.MetricsExporter(ctx)

	queryIDs, ok := r.URL.Query()[queryParam]
	if !ok {
		metrics.WriteInt("federation-pull-invalid-request", true, 1)
		badRequestf(ctx, w, "%s is required", queryParam)
		return
	}
	if len(queryIDs) > 1 {
		metrics.WriteInt("federation-pull-invalid-request", true, 1)
		badRequestf(ctx, w, "only one %s allowed", queryParam)
		return
	}
	queryID := queryIDs[0]
	if queryID == "" {
		metrics.WriteInt("federation-pull-invalid-request", true, 1)
		badRequestf(ctx, w, "%s is required", queryParam)
		return
	}

	// Obtain lock to make sure there are no other processes working on this batch.
	lock := "query_" + queryID
	unlockFn, err := h.db.Lock(ctx, lock, h.config.Timeout)
	if err != nil {
		if errors.Is(err, coredb.ErrAlreadyLocked) {
			metrics.WriteInt("federation-pull-lock-contention", true, 1)
			msg := fmt.Sprintf("Lock %s already in use. No work will be performed.", lock)
			logger.Infof(msg)
			w.Write([]byte(msg)) // We return status 200 here so that Cloud Scheduler does not retry.
			return
		}
		internalErrorf(ctx, w, "Could not acquire lock %s for query %s: %v", lock, queryID, err)
		return
	}
	defer unlockFn()

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
	dialOpts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	var clientOpts []idtoken.ClientOption
	if h.config.CredentialsFile != "" {
		clientOpts = append(clientOpts, idtoken.WithCredentialsFile(h.config.CredentialsFile))
	}
	ts, err := idtoken.NewTokenSource(ctx, query.Audience, clientOpts...)
	if err != nil {
		internalErrorf(ctx, w, "Failed to create token source: %v", err)
		return
	}
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(oauth.TokenSource{ts}))

	logger.Infof("Dialing %s", query.ServerAddr)
	conn, err := grpc.Dial(query.ServerAddr, dialOpts...)
	if err != nil {
		internalErrorf(ctx, w, "Failed to dial for query %q %s: %v", queryID, query.ServerAddr, err)
		return
	}
	defer conn.Close()
	client := pb.NewFederationClient(conn)

	timeoutContext, cancel := context.WithTimeout(ctx, h.config.Timeout)
	defer cancel()

	deps := pullDependencies{
		fetch:               client.Fetch,
		insertExposures:     h.publishdb.InsertExposures,
		startFederationSync: h.db.StartFederationInSync,
	}
	batchStart := time.Now()
	if err := pull(timeoutContext, metrics, deps, query, batchStart, h.config.TruncateWindow); err != nil {
		internalErrorf(ctx, w, "Federation query %q failed: %v", queryID, err)
		return
	}

	if timeoutContext.Err() != nil && timeoutContext.Err() == context.DeadlineExceeded {
		logger.Infof("Federation puller timed out at %v before fetching entire set.", h.config.Timeout)
	}
}

func pull(ctx context.Context, metrics metrics.Exporter, deps pullDependencies, q *coredb.FederationInQuery, batchStart time.Time, truncateWindow time.Duration) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing query %q", q.QueryID)

	request := &pb.FederationFetchRequest{
		RegionIdentifiers:             q.IncludeRegions,
		ExcludeRegionIdentifiers:      q.ExcludeRegions,
		LastFetchResponseKeyTimestamp: q.LastTimestamp.Unix(),
	}

	syncID, finalizeFn, err := deps.startFederationSync(ctx, q, batchStart)
	if err != nil {
		return fmt.Errorf("starting federation sync for query %s: %w", q.QueryID, err)
	}

	var maxTimestamp time.Time
	total := 0
	defer func() {
		logger.Infof("Inserted %d keys", total)
	}()

	createdAt := model.TruncateWindow(batchStart, truncateWindow)
	partial := true
	for partial {

		// TODO(squee1945): react to the context timeout and complete a chunk of work so next invocation can pick up where left off.

		response, err := deps.fetch(ctx, request)
		if err != nil {
			return fmt.Errorf("fetching query %s: %w", q.QueryID, err)
		}

		responseTimestamp := time.Unix(response.FetchResponseKeyTimestamp, 0)
		if responseTimestamp.After(maxTimestamp) {
			maxTimestamp = responseTimestamp
		}

		// Loop through the result set, storing in database.
		var exposures []*model.Exposure
		for _, ctr := range response.Response {

			var upperRegions []string
			for _, region := range ctr.RegionIdentifiers {
				upperRegions = append(upperRegions, strings.ToUpper(strings.TrimSpace(region)))
			}
			sort.Strings(upperRegions)

			for _, cti := range ctr.ContactTracingInfo {
				for _, key := range cti.ExposureKeys {

					if cti.TransmissionRisk < model.MinTransmissionRisk || cti.TransmissionRisk > model.MaxTransmissionRisk {
						logger.Errorf("invalid transmission risk %v - dropping record.", cti.TransmissionRisk)
						continue
					}

					exposures = append(exposures, &model.Exposure{
						TransmissionRisk: int(cti.TransmissionRisk),
						ExposureKey:      key.ExposureKey,
						Regions:          upperRegions,
						FederationSyncID: syncID,
						IntervalNumber:   key.IntervalNumber,
						IntervalCount:    key.IntervalCount,
						CreatedAt:        createdAt,
						LocalProvenance:  false,
					})

					if len(exposures) == fetchBatchSize {
						if err := deps.insertExposures(ctx, exposures); err != nil {
							metrics.WriteInt("federation-pull-inserts", false, len(exposures))
							return fmt.Errorf("inserting %d exposures: %w", len(exposures), err)
						}
						total += len(exposures)
						exposures = nil // Start a new batch.
					}
				}
			}
		}
		if len(exposures) > 0 {
			if err := deps.insertExposures(ctx, exposures); err != nil {
				metrics.WriteInt("federation-pull-inserts", false, len(exposures))
				return fmt.Errorf("inserting %d exposures: %w", len(exposures), err)
			}
			total += len(exposures)
		}

		partial = response.PartialResponse
		request.NextFetchToken = response.NextFetchToken
	}

	if err := finalizeFn(maxTimestamp, total); err != nil {
		// TODO(squee1945): how do we clean up here? Just leave the records in and have the exporter eliminate them? Other?
		return fmt.Errorf("finalizing federation sync for query %s: %w", q.QueryID, err)
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
