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

// Package export defines the handlers for managing exposure key exporting.
package export

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
)

// CreateBatchesHandler is a handler to iterate the rows of ExportConfig and
// create entries in ExportBatchJob as appropriate.
func (s *Server) CreateBatchesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.CreateTimeout)
	defer cancel()
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)

	// Obtain lock to make sure there are no other processes working to create batches.
	lock := "create_batches"
	unlockFn, err := s.db.Lock(ctx, lock, s.config.CreateTimeout)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
			metrics.WriteInt("export-batcher-lock-contention", true, 1)
			msg := fmt.Sprintf("Lock %s already in use, no work will be performed", lock)
			logger.Infof(msg)
			w.Write([]byte(msg)) // We return status 200 here so that Cloud Scheduler does not retry.
			return
		}
		logger.Errorf("Could not acquire lock %s: %v", lock, err)
		http.Error(w, fmt.Sprintf("Could not acquire lock %s, check logs.", lock), http.StatusInternalServerError)
		return
	}
	defer unlockFn()

	totalConfigs := 0
	totalBatches := 0
	totalConfigsWithBatches := 0
	defer func() {
		logger.Infof("Processed %d configs creating %d batches across %d configs", totalConfigs, totalBatches, totalConfigsWithBatches)
	}()

	now := time.Now()
	err = s.db.IterateExportConfigs(ctx, now, func(ec *model.ExportConfig) error {
		totalConfigs++
		if batchesCreated, err := s.maybeCreateBatches(ctx, ec, now); err != nil {
			logger.Errorf("Failed to create batches for config %d: %v, continuing to next config", ec.ConfigID, err)
		} else {
			totalBatches += batchesCreated
			if batchesCreated > 0 {
				totalConfigsWithBatches++
			}
		}
		return nil
	})
	if err != nil {
		// some specific error handling below, but just need one metric.
		metrics.WriteInt("export-batcher-failed", true, 1)
	}
	switch {
	case err == nil:
		return
	case errors.Is(err, context.DeadlineExceeded):
		logger.Infof("Timed out creating batches, batch creation will continue on next invocation")
	case errors.Is(err, context.Canceled):
		logger.Infof("Canceled while creating batches, batch creation will continue on next invocation")
	default:
		logger.Errorf("creating batches: %v", err)
		http.Error(w, "Failed to create batches, check logs.", http.StatusInternalServerError)
	}
}

func (s *Server) maybeCreateBatches(ctx context.Context, ec *model.ExportConfig, now time.Time) (int, error) {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)

	latestEnd, err := s.db.LatestExportBatchEnd(ctx, ec)
	if err != nil {
		return 0, fmt.Errorf("fetching most recent batch for config %d: %w", ec.ConfigID, err)
	}

	ranges := makeBatchRanges(ec.Period, latestEnd, now, s.config.TruncateWindow)
	if len(ranges) == 0 {
		metrics.WriteInt("export-batcher-no-work", true, 1)
		logger.Debugf("Batch creation for config %d is not required, skipping", ec.ConfigID)
		return 0, nil
	}

	var batches []*model.ExportBatch
	for _, br := range ranges {
		infoIds := make([]int64, len(ec.SignatureInfoIDs))
		copy(infoIds, ec.SignatureInfoIDs)
		batches = append(batches, &model.ExportBatch{
			ConfigID:         ec.ConfigID,
			BucketName:       ec.BucketName,
			FilenameRoot:     ec.FilenameRoot,
			StartTimestamp:   br.start,
			EndTimestamp:     br.end,
			Region:           ec.Region,
			Status:           model.ExportBatchOpen,
			SignatureInfoIDs: infoIds,
		})
	}

	if err := s.db.AddExportBatches(ctx, batches); err != nil {
		return 0, fmt.Errorf("creating export batches for config %d: %w", ec.ConfigID, err)
	}

	metrics.WriteInt("export-batcher-created", true, len(batches))
	logger.Infof("Created %d batch(es) for config %d", len(batches), ec.ConfigID)
	return len(batches), nil
}

type batchRange struct {
	start, end time.Time
}

var sanityDate = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func makeBatchRanges(period time.Duration, latestEnd, now time.Time, truncateWindow time.Duration) []batchRange {

	// Compute the end of the exposure publish window; we don't want any batches with an end date greater than this time.
	publishEnd := model.TruncateWindow(now, truncateWindow)

	// Special case: if there have not been batches before, return only a single one.
	// We use sanityDate here because the loop below will happily create batch ranges
	// until the beginning of time otherwise.
	if latestEnd.Before(sanityDate) {
		// We want to create a batch aligned on the period, but not overlapping the current publish window.
		// To do this, we use the publishEnd and truncate it to the period; this becomes the end date.
		// Then we just subtract the period to get the start date.
		end := publishEnd.Truncate(period)
		start := end.Add(-period)
		return []batchRange{{start: start, end: end}}
	}

	// Truncate now to align with period; use this as the end date.
	end := now.Truncate(period)

	// If the end date < latest end date, we already have a batch that covers this period, so return no batches.
	if end.Before(latestEnd) {
		return nil
	}

	// Subtract period to get the start date.
	start := end.Add(-period)

	// Build up a list of batches until we reach that latestEnd.
	// Allow for overlap so we don't miss keys; this might happen in the event that
	// an ExportConfig was edited and the new settings don't quite align.
	ranges := []batchRange{}
	for end.After(latestEnd) {
		// If the batch's end is after the publish window, don't add this range.
		if !end.After(publishEnd) {
			ranges = append([]batchRange{{start: start, end: end}}, ranges...)
		}
		start = start.Add(-period)
		end = end.Add(-period)
	}
	return ranges
}
