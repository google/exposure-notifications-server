package api

import (
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"context"
	"fmt"
	"net/http"
	"time"

	"google.golang.org/grpc"
)

const (
	queryParam = "query_id"
)

// HandleFederationPull returns a handler that will fetch server-to-server federation results for a single federation query.
func HandleFederationPull(timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		query, err := database.GetFederationQuery(ctx, queryID)
		if err != nil {
			if err == database.ErrQueryNotFound {
				http.Error(w, fmt.Sprintf("unknown %s", queryParam), http.StatusBadRequest)
				return
			}
			logger.Errorf("Failed getting query %q: %v", queryID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		timeoutContext, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		if err := pull(timeoutContext, query, timeout); err != nil {
			logger.Errorf("Federation query %q failed: %v", queryID, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		if timeoutContext.Err() != nil && timeoutContext.Err() == context.DeadlineExceeded {
			logger.Infof("Federation puller timed out at %v before fetching entire set.", timeout)
		}
	}
}

func pull(ctx context.Context, query *model.FederationQuery, timeout time.Duration) error {
	logger := logging.FromContext(ctx)
	queryID := query.K.String()

	logger.Infof("Processing query %q", queryID)

	// Obtain lock to make sure there are no other processes working on this batch.
	lock := "query_" + queryID
	unlockFn, err := database.Lock(ctx, lock, timeout)
	if err != nil {
		if err == database.ErrAlreadyLocked {
			logger.Infof("Lock %s already in use. No work will be performed.", lock)
			return nil
		}
		return fmt.Errorf("could not acquire lock %s: %v", lock, err)
	}
	defer unlockFn()

	// TODO(jasonco): make secure
	conn, err := grpc.Dial(query.ServerAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("dialing %s: %v", query.ServerAddr, err)
	}
	defer conn.Close()
	client := pb.NewFederationClient(conn)

	request := &pb.FederationFetchRequest{
		RegionIdentifiers:             query.IncludeRegions,
		ExcludeRegionIdentifiers:      query.ExcludeRegions,
		LastFetchResponseKeyTimestamp: query.LastTimestamp.Unix(),
	}

	total := 0
	defer func() {
		logger.Infof("Inserted %d keys", total)
	}()

	syncID, finalizeFn, err := database.StartFederationSync(ctx, query)
	if err != nil {
		return fmt.Errorf("starting federation sync for query %s: %v", queryID, err)
	}

	var maxTimestamp time.Time
	partial := true
	for partial {

		// TODO(jasonco): react to the context timeout and complete a chunk of work so next invocation can pick up where left off.

		response, err := client.Fetch(ctx, request)
		if err != nil {
			return fmt.Errorf("fetching query %s: %v", queryID, err)
		}

		responseTimestamp := time.Unix(response.FetchResponseKeyTimestamp, 0)
		if responseTimestamp.After(maxTimestamp) {
			maxTimestamp = responseTimestamp
		}

		// Loop through the result set, storing in database.
		// TODO(jasonco): ideally there would be a transaction for every change of date window. This will allow us to recover cleanly.
		// TODO(jasonco): datastore has batch size limitations that need to be considered; this won't interplay well with above TODO.
		var infections []model.Infection
		for _, ctr := range response.Response {
			for _, cti := range ctr.ContactTracingInfo {
				for _, key := range cti.DiagnosisKeys {
					infections = append(infections, model.Infection{
						DiagnosisKey: key.DiagnosisKey,
						// AppPackageName: "",
						Regions: ctr.RegionIdentifiers,
						// Platform:         "",
						FederationSyncId: syncID,
						KeyDay:           time.Unix(key.Timestamp, 0),
						CreatedAt:        model.TruncateWindow(time.Now()), // TODO(jasonco): should this be now, or the time this batch started? Should it be truncated at all?
					})

					if len(infections) == database.InsertInfectionsBatchSize {
						if err := database.InsertInfections(ctx, infections); err != nil {
							// This is challenging without transactions. Which were inserted, which we not? We don't have idempotent keys, so we can insert duplciates.
							// Duplicate keys are probably not bad, so maybe we should just start fetching at the start of this overall request (on the next invocation).
							return fmt.Errorf("inserting %d infections: %v", len(infections), err)
						}
						total += len(infections)
						infections = nil // Start a new batch.
					}
				}
			}
		}
		if len(infections) > 0 {
			if err := database.InsertInfections(ctx, infections); err != nil {
				// This is challenging without transactions. Which were inserted, which we not? We don't have idempotent keys, so we can insert duplciates.
				// Duplicate keys are probably not bad, so maybe we should just start fetching at the start of this overall request (on the next invocation).
				return fmt.Errorf("inserting %d infections: %v", len(infections), err)
			}
			total += len(infections)
		}

		partial = response.PartialResponse
		request.FetchToken = response.NextFetchToken
	}

	if err := finalizeFn(time.Now(), maxTimestamp, total); err != nil {
		// TODO(jasonco): how do we clean up here? Just leave the records in and have the exporter eliminate them? Other?
		return fmt.Errorf("finalizing federation sync for query %s: %v", queryID, err)
	}

	return nil
}
