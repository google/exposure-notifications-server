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

package export

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	exportdatabase "github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"go.opencensus.io/stats"

	"github.com/google/exposure-notifications-server/pkg/logging"
)

const createBatchesLock = "create_batches"

// handleCreateBatches is a handler to iterate the rows of ExportConfig and
// create entries in ExportBatchJob as appropriate.
func (s *Server) handleCreateBatches() http.Handler {
	db := s.env.Database()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handleCreateBatches")

		ctx, cancel := context.WithTimeout(ctx, s.config.CreateTimeout)
		defer cancel()

		// Obtain lock to make sure there are no other processes working to create batches.
		unlockFn, err := db.Lock(ctx, createBatchesLock, s.config.CreateTimeout)
		if err != nil {
			if errors.Is(err, database.ErrAlreadyLocked) {
				stats.Record(ctx, mBatcherLockContention.M(1))
				logger.Infow("already locked")
				w.WriteHeader(http.StatusOK)
				return
			}

			logger.Errorw("failed to lock", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := unlockFn(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}()

		totalConfigs := 0
		totalBatches := 0
		totalConfigsWithBatches := 0
		defer logger.Debugw("finished",
			"configs", totalConfigs,
			"batches", totalBatches,
			"total", totalConfigsWithBatches)

		effectiveTime := time.Now().Add(-1 * s.config.MinWindowAge)
		if err := exportdatabase.New(db).IterateExportConfigs(ctx, effectiveTime, func(ec *model.ExportConfig) error {
			totalConfigs++
			batchesCreated, err := s.maybeCreateBatches(ctx, ec, effectiveTime)
			if err != nil {
				logger.Errorw("failed to create batches", "config", ec.ConfigID, "error", err)
				return nil
			}

			totalBatches += batchesCreated
			if batchesCreated > 0 {
				totalConfigsWithBatches++
			}
			return nil
		}); err != nil {
			// some specific error handling below, but just need one metric.
			stats.Record(ctx, mBatcherFailure.M(1))

			switch {
			case errors.Is(err, context.DeadlineExceeded):
				logger.Infow("batch creation timed out")
				w.WriteHeader(http.StatusOK)
				return
			case errors.Is(err, context.Canceled):
				logger.Infow("batch creation canceled")
				w.WriteHeader(http.StatusOK)
				return
			default:
				logger.Errorw("failed to create batches", "error", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	})
}

func (s *Server) maybeCreateBatches(ctx context.Context, ec *model.ExportConfig, now time.Time) (int, error) {
	logger := logging.FromContext(ctx).Named("maybeCreateBatches").
		With("config", ec.ConfigID)
	exportDB := exportdatabase.New(s.env.Database())

	latestEnd, err := exportDB.LatestExportBatchEnd(ctx, ec)
	if err != nil {
		return 0, fmt.Errorf("fetching most recent batch for config %d: %w", ec.ConfigID, err)
	}

	ranges := makeBatchRanges(ec.Period, latestEnd, now, s.config.TruncateWindow)
	if len(ranges) == 0 {
		stats.Record(ctx, mBatcherNoWork.M(1))
		logger.Debugw("skipping batch creation")
		return 0, nil
	}

	var batches []*model.ExportBatch
	for _, br := range ranges {
		infoIds := make([]int64, len(ec.SignatureInfoIDs))
		copy(infoIds, ec.SignatureInfoIDs)
		batches = append(batches, &model.ExportBatch{
			ConfigID:           ec.ConfigID,
			BucketName:         ec.BucketName,
			FilenameRoot:       ec.FilenameRoot,
			StartTimestamp:     br.start,
			EndTimestamp:       br.end,
			OutputRegion:       ec.OutputRegion,
			InputRegions:       ec.InputRegions,
			IncludeTravelers:   ec.IncludeTravelers,
			OnlyNonTravelers:   ec.OnlyNonTravelers,
			ExcludeRegions:     ec.ExcludeRegions,
			Status:             model.ExportBatchOpen,
			SignatureInfoIDs:   infoIds,
			MaxRecordsOverride: ec.MaxRecordsOverride,
		})
	}

	if err := exportDB.AddExportBatches(ctx, batches); err != nil {
		return 0, fmt.Errorf("creating export batches for config %d: %w", ec.ConfigID, err)
	}

	stats.Record(ctx, mBatcherCreated.M(int64(len(batches))))
	logger.Debugw("created batches", "batches", len(batches))
	return len(batches), nil
}

type batchRange struct {
	start, end time.Time
}

var sanityDate = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func makeBatchRanges(period time.Duration, latestEnd, now time.Time, truncateWindow time.Duration) []batchRange {
	// Compute the end of the exposure publish window; we don't want any batches with an end date greater than this time.
	publishEnd := publishmodel.TruncateWindow(now, truncateWindow)

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
