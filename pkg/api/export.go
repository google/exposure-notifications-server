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

// Package api defines the structures for the infection publishing API.
package api

import (
	"encoding/json"
	"net/http"

	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/model"
	"cambio/pkg/storage"
	"context"
	"fmt"
	"strconv"
	"time"
)

const (
	batchIDParam = "batch-id"
)

// CreateBatchesHandler is a handler to iterate the rows of ExportConfig and create entries in ExportBatchJob as appropriate.
type CreateBatchesHandler struct {
	Timeout time.Duration
}

func (h CreateBatchesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.Timeout)
	defer cancel()
	logger := logging.FromContext(ctx)

	// Obtain lock to make sure there are no other processes working to create batches.
	lock := "create_batches"
	unlockFn, err := database.Lock(ctx, lock, h.Timeout) // TODO(jasonco): double this?
	if err != nil {
		if err == database.ErrAlreadyLocked {
			msg := fmt.Sprintf("Lock %s already in use. No work will be performed.", lock)
			logger.Infof(msg)
			w.Write([]byte(msg)) // We return status 200 here so that Cloud Scheduler does not retry.
			return
		}
		logger.Errorf("Could not acquire lock %s: %v", lock, err)
		http.Error(w, fmt.Sprintf("Could not acquire lock %s, check logs.", lock), http.StatusInternalServerError)
		return
	}
	defer unlockFn()

	now := time.Now().UTC()
	it, err := database.IterateExportConfigs(ctx, now)
	if err != nil {
		logger.Errorf("Failed to get export config iterator: %v", err)
		http.Error(w, "Failed to get export config iterator, check logs.", http.StatusInternalServerError)
		return
	}
	defer it.Close()

	done := false
	for !done {

		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled { // May be context.Canceled due to test code.
				logger.Errorf("Context error: %v", err)
				return
			}
			logger.Infof("Timed out before iterating batches. Will pick up on next invocation.")
			return

		default:
			// Fallthrough to process a record.
		}

		var ec *model.ExportConfig
		var err error
		ec, done, err = it.Next()
		if err != nil {
			logger.Errorf("Failed to iterate export config: %v", err)
			http.Error(w, "Failed to iterate export config, check logs.", http.StatusInternalServerError)
			return
		}
		if done {
			return
		}
		if ec == nil {
			continue
		}

		if err := maybeCreateBatch(ctx, ec, now); err != nil {
			logger.Errorf("Failed to create batch for config %d: %v. Continuing", ec.ConfigID, err)
		}
	}
}

func maybeCreateBatch(ctx context.Context, ec *model.ExportConfig, now time.Time) error {
	logger := logging.FromContext(ctx)

	latestStart, err := database.LatestExportBatchStartTime(ctx, ec)
	if err != nil {
		return fmt.Errorf("fetching most recent batch for config %d: %v", ec.ConfigID, err)
	}

	newStartTimestamp := now.Truncate(ec.Period)
	create := false
	if latestStart == nil {
		create = true
	} else {
		if latestStart.Before(newStartTimestamp) {
			create = true
		}
	}

	if !create {
		logger.Debugf("Batch creation for config %d is not required. Skipping.", ec.ConfigID)
		return nil
	}

	eb := model.ExportBatch{
		ConfigID:       ec.ConfigID,
		FilenameRoot:   ec.FilenameRoot,
		StartTimestamp: newStartTimestamp,
		EndTimestamp:   newStartTimestamp.Add(ec.Period),
		IncludeRegions: ec.IncludeRegions,
		ExcludeRegions: ec.ExcludeRegions,
		Status:         model.ExportBatchOpen,
	}
	if err := database.AddExportBatch(ctx, &eb); err != nil {
		return fmt.Errorf("creating export batch for config %d: %v", ec.ConfigID, err)
	}

	logger.Infof("Created batch %d for config %d.", eb.BatchID, ec.ConfigID)
	return nil
}

// LeaseBatchHandler leases and returns a batch job for the worker to process. Returns a 404 if no work to do.
func LeaseBatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	ttl := 15 * time.Minute // TODO(jasonco): take from args?
	batch, err := database.LeaseBatch(ctx, ttl, time.Now().UTC())
	if err != nil {
		logger.Errorf("Failed to lease batch: %v", err)
		http.Error(w, "Failed to lease batch, check logs.", http.StatusInternalServerError)
		return
	}

	if batch == nil {
		logger.Debugf("No work to do.")
		http.Error(w, "No work to do.", http.StatusNotFound)
		return
	}

	b, err := json.Marshal(&batch)
	if err != nil {
		logger.Errorf("Failed to marshal JSON: %v", err)
		http.Error(w, "Failed to marshal JSON, check logs.", http.StatusInternalServerError)
	}

	n, err := w.Write(b)
	if err != nil || n < len(b) {
		logger.Errorf("Failed to write JSON: %v", err)
		http.Error(w, "Failed to write JSON, check logs.", http.StatusInternalServerError)
	}
}

// CompleteBatchHandler marks a batch job as completed.
func CompleteBatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	batchIDs, ok := r.URL.Query()[batchIDParam]
	if !ok {
		http.Error(w, fmt.Sprintf("%s is required", batchIDParam), http.StatusBadRequest)
		return
	}
	if len(batchIDs) > 1 {
		http.Error(w, fmt.Sprintf("only one %s allowed", batchIDParam), http.StatusBadRequest)
		return
	}
	batchIDStr := batchIDs[0]
	if batchIDStr == "" {
		http.Error(w, fmt.Sprintf("%s is required", batchIDParam), http.StatusBadRequest)
		return
	}
	batchID, err := strconv.ParseInt(batchIDStr, 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s must be an integer", batchIDParam), http.StatusBadRequest)
		return
	}

	if err := database.CompleteBatch(ctx, batchID); err != nil {
		if err == database.ErrNotFound {
			http.Error(w, fmt.Sprintf("%d not found", batchID), http.StatusBadRequest)
			return
		}
		logger.Errorf("Failed to mark batch %d complete: %v", batchID, err)
		http.Error(w, fmt.Sprintf("Unexpected error, see logs."), http.StatusInternalServerError)
		return
	}
	w.Write([]byte(fmt.Sprintf("Batch %d marked completed", batchID)))
}

func TestExportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	limit := 30000
	limits, ok := r.URL.Query()["limit"]
	if ok && len(limits) > 0 {
		lim, err := strconv.Atoi(limits[0])
		if err == nil {
			limit = lim
		}
	}
	logger.Infof("limiting to %v", limit)
	since := time.Now().UTC().AddDate(0, 0, -5)
	until := time.Now().UTC()
	exposureKeys, err := queryExposureKeys(ctx, since, until, limit)
	if err != nil {
		logger.Errorf("error getting infections: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
	}
	data, err := MarshalExportFile(since, until, exposureKeys, "US")
	if err != nil {
		logger.Errorf("error marshalling export file: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
	}
	objectName := fmt.Sprintf("testExport-%d-records.pb", limit)
	if err := storage.CreateObject("apollo-public-bucket", objectName, data); err != nil {
		logger.Errorf("error creating cloud storage object: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func queryExposureKeys(ctx context.Context, since, until time.Time, limit int) ([]*model.Infection, error) {
	criteria := database.IterateInfectionsCriteria{
		SinceTimestamp:      since,
		UntilTimestamp:      until,
		OnlyLocalProvenance: false, // include federated ids
	}
	it, err := database.IterateInfections(ctx, criteria)
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var exposureKeys []*model.Infection
	num := 1
	exp, done, err := it.Next()
	for !done && err == nil && num <= limit {
		if exp != nil {
			exposureKeys = append(exposureKeys, exp)
			num++
		}
		exp, done, err = it.Next()
	}
	if err != nil {
		return nil, err
	}
	return exposureKeys, nil
}
