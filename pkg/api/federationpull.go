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

package api

import (
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
)

const (
	queryParam = "query-id"
)

var (
	fetchBatchSize = database.InsertInfectionsBatchSize
)

type fetchFn func(context.Context, *pb.FederationFetchRequest, ...grpc.CallOption) (*pb.FederationFetchResponse, error)
type insertInfectionsFn func(context.Context, []*model.Infection) error
type startFederationSyncFn func(context.Context, *model.FederationQuery, time.Time) (string, database.FinalizeSyncFn, error)

type pullDependencies struct {
	fetch               fetchFn
	insertInfections    insertInfectionsFn
	startFederationSync startFederationSyncFn
}

// NewFederationPullHandler returns a handler that will fetch server-to-server
// federation results for a single federation query.
func NewFederationPullHandler(db *database.DB, timeout time.Duration) http.Handler {
	return &federationPullHandler{db: db, timeout: timeout}
}

type federationPullHandler struct {
	db      *database.DB
	timeout time.Duration
}

func (h *federationPullHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	queryIDs, ok := r.URL.Query()[queryParam]
	if !ok {
		http.Error(w, fmt.Sprintf("%s is required", queryParam), http.StatusBadRequest)
		return
	}
	if len(queryIDs) > 1 {
		http.Error(w, fmt.Sprintf("only one %s allowed", queryParam), http.StatusBadRequest)
		return
	}
	queryID := queryIDs[0]
	if queryID == "" {
		http.Error(w, fmt.Sprintf("%s is required", queryParam), http.StatusBadRequest)
		return
	}

	query, err := h.db.GetFederationQuery(ctx, queryID)
	if err != nil {
		if err == database.ErrNotFound {
			http.Error(w, fmt.Sprintf("unknown %s", queryParam), http.StatusBadRequest)
			return
		}
		logger.Errorf("Failed getting query %q: %v", queryID, err)
		http.Error(w, fmt.Sprintf("Failed getting query %q, check logs.", queryID), http.StatusInternalServerError)
		return
	}

	// Obtain lock to make sure there are no other processes working on this batch.
	lock := "query_" + queryID
	unlockFn, err := h.db.Lock(ctx, lock, h.timeout)
	if err != nil {
		if err == database.ErrAlreadyLocked {
			msg := fmt.Sprintf("Lock %s already in use. No work will be performed.", lock)
			logger.Infof(msg)
			w.Write([]byte(msg)) // We return status 200 here so that Cloud Scheduler does not retry.
			return
		}
		logger.Errorf("Could not acquire lock %s for query %s: %v", lock, queryID, err)
		http.Error(w, fmt.Sprintf("Could not acquire lock %s for query %s, check logs.", lock, queryID), http.StatusInternalServerError)
		return
	}
	defer unlockFn()

	// TODO(jasonco): make secure
	conn, err := grpc.Dial(query.ServerAddr, grpc.WithInsecure())
	if err != nil {
		logger.Errorf("Failed to dial for query %q %s: %v", queryID, query.ServerAddr, err)
		http.Error(w, fmt.Sprintf("Failed to dial for query %q, check logs.", queryID), http.StatusInternalServerError)
	}
	defer conn.Close()
	client := pb.NewFederationClient(conn)

	timeoutContext, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	deps := pullDependencies{
		fetch:               client.Fetch,
		insertInfections:    h.db.InsertInfections,
		startFederationSync: h.db.StartFederationSync,
	}
	batchStart := time.Now().UTC()
	if err := federationPull(timeoutContext, deps, query, batchStart); err != nil {
		logger.Errorf("Federation query %q failed: %v", queryID, err)
		http.Error(w, fmt.Sprintf("Federation query %q fetch failed, check logs.", queryID), http.StatusInternalServerError)
	}

	if timeoutContext.Err() != nil && timeoutContext.Err() == context.DeadlineExceeded {
		logger.Infof("Federation puller timed out at %v before fetching entire set.", h.timeout)
	}
}

func federationPull(ctx context.Context, deps pullDependencies, q *model.FederationQuery, batchStart time.Time) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing query %q", q.QueryID)

	request := &pb.FederationFetchRequest{
		RegionIdentifiers:             q.IncludeRegions,
		ExcludeRegionIdentifiers:      q.ExcludeRegions,
		LastFetchResponseKeyTimestamp: q.LastTimestamp.Unix(),
	}

	syncID, finalizeFn, err := deps.startFederationSync(ctx, q, batchStart)
	if err != nil {
		return fmt.Errorf("starting federation sync for query %s: %v", q.QueryID, err)
	}

	var maxTimestamp time.Time
	total := 0
	defer func() {
		logger.Infof("Inserted %d keys", total)
	}()

	createdAt := model.TruncateWindow(batchStart)
	partial := true
	for partial {

		// TODO(jasonco): react to the context timeout and complete a chunk of work so next invocation can pick up where left off.

		response, err := deps.fetch(ctx, request)
		if err != nil {
			return fmt.Errorf("fetching query %s: %v", q.QueryID, err)
		}

		responseTimestamp := time.Unix(response.FetchResponseKeyTimestamp, 0).UTC()
		if responseTimestamp.After(maxTimestamp) {
			maxTimestamp = responseTimestamp
		}

		// Loop through the result set, storing in database.
		var infections []*model.Infection
		for _, ctr := range response.Response {

			var upperRegions []string
			for _, region := range ctr.RegionIdentifiers {
				upperRegions = append(upperRegions, strings.ToUpper(strings.TrimSpace(region)))
			}
			sort.Strings(upperRegions)

			for _, cti := range ctr.ContactTracingInfo {

				verificationAuthName := strings.ToUpper(strings.TrimSpace(cti.VerificationAuthorityName))

				for _, key := range cti.ExposureKeys {

					infections = append(infections, &model.Infection{
						TransmissionRisk:          int(cti.TransmissionRisk),
						ExposureKey:               key.ExposureKey,
						Regions:                   upperRegions,
						FederationSyncID:          syncID,
						IntervalNumber:            key.IntervalNumber,
						IntervalCount:             key.IntervalCount,
						CreatedAt:                 createdAt,
						LocalProvenance:           false,
						VerificationAuthorityName: verificationAuthName,
					})

					if len(infections) == fetchBatchSize {
						if err := deps.insertInfections(ctx, infections); err != nil {
							return fmt.Errorf("inserting %d infections: %v", len(infections), err)
						}
						total += len(infections)
						infections = nil // Start a new batch.
					}
				}
			}
		}
		if len(infections) > 0 {
			if err := deps.insertInfections(ctx, infections); err != nil {
				return fmt.Errorf("inserting %d infections: %v", len(infections), err)
			}
			total += len(infections)
		}

		partial = response.PartialResponse
		request.NextFetchToken = response.NextFetchToken
	}

	if err := finalizeFn(maxTimestamp, total); err != nil {
		// TODO(jasonco): how do we clean up here? Just leave the records in and have the exporter eliminate them? Other?
		return fmt.Errorf("finalizing federation sync for query %s: %v", q.QueryID, err)
	}

	return nil
}
