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

// Package export defines the handlers for managing exporure key exporting
package export

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	filenameSuffix       = ".zip"
	blobOperationTimeout = 50 * time.Second
)

// NewBatchServer makes a BatchServer.
func NewBatchServer(db *database.DB, bsc BatchServerConfig, env *serverenv.ServerEnv) (*BatchServer, error) {
	// Validate config.
	if env.Blobstore == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires Blobstore present in the ServerEnv")
	}
	if env.KeyManager == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires KeyMenger present in the ServerEnv")
	}

	return &BatchServer{
		db:  db,
		bsc: bsc,
		env: env,
	}, nil
}

// BatchServer hosts end points to manage export batches.
type BatchServer struct {
	db  *database.DB
	bsc BatchServerConfig
	env *serverenv.ServerEnv
}

// BatchServerConfig is configuratiion for a BatchServer.
type BatchServerConfig struct {
	CreateTimeout time.Duration
	WorkerTimeout time.Duration
	MaxRecords    int
}

// CreateBatchesHandler is a handler to iterate the rows of ExportConfig and
// create entries in ExportBatchJob as appropriate.
func (s *BatchServer) CreateBatchesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.bsc.CreateTimeout)
	defer cancel()
	logger := logging.FromContext(ctx)

	// Obtain lock to make sure there are no other processes working to create batches.
	lock := "create_batches"
	unlockFn, err := s.db.Lock(ctx, lock, s.bsc.CreateTimeout)
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
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

	now := time.Now()
	err = s.db.IterateExportConfigs(ctx, now, func(ec *model.ExportConfig) error {
		if err := s.maybeCreateBatches(ctx, ec, now); err != nil {
			logger.Errorf("Failed to create batches for config %d: %v, continuing to next config", ec.ConfigID, err)
		}
		return nil
	})
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

func (s *BatchServer) maybeCreateBatches(ctx context.Context, ec *model.ExportConfig, now time.Time) error {
	logger := logging.FromContext(ctx)

	latestEnd, err := s.db.LatestExportBatchEnd(ctx, ec)
	if err != nil {
		return fmt.Errorf("fetching most recent batch for config %d: %w", ec.ConfigID, err)
	}

	ranges := makeBatchRanges(ec.Period, latestEnd, now)
	if len(ranges) == 0 {
		logger.Infof("Batch creation for config %d is not required, skipping", ec.ConfigID)
		return nil
	}

	var batches []*model.ExportBatch
	for _, br := range ranges {
		batches = append(batches, &model.ExportBatch{
			ConfigID:       ec.ConfigID,
			BucketName:     ec.BucketName,
			FilenameRoot:   ec.FilenameRoot,
			StartTimestamp: br.start,
			EndTimestamp:   br.end,
			Region:         ec.Region,
			Status:         model.ExportBatchOpen,
			SigningKey:     ec.SigningKey,
		})
	}

	if err := s.db.AddExportBatches(ctx, batches); err != nil {
		return fmt.Errorf("creating export batches for config %d: %w", ec.ConfigID, err)
	}

	logger.Infof("Created %d batch(es) for config %d", len(batches), ec.ConfigID)
	return nil
}

type batchRange struct {
	start, end time.Time
}

var sanityDate = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func makeBatchRanges(period time.Duration, latestEnd, now time.Time) []batchRange {

	// Compute the end of the exposure publish window; we don't want any batches with an end date greater than this time.
	publishEnd := model.TruncateWindow(now)

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

// WorkerHandler is a handler to iterate the rows of ExportBatch, and creates GCS files.
func (s *BatchServer) WorkerHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.bsc.WorkerTimeout)
	defer cancel()
	logger := logging.FromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				logger.Errorf("Context error while processing batches: %v", err)
				return
			}
			msg := "Timed out processing batches. Will continue on next invocation."
			logger.Info(msg)
			fmt.Fprintln(w, msg)
			return
		default:
			// Fallthrough
		}

		// Check for a batch and obtain a lease for it.
		batch, err := s.db.LeaseBatch(ctx, s.bsc.WorkerTimeout, time.Now())
		if err != nil {
			logger.Errorf("Failed to lease batch: %v", err)
			continue
		}
		if batch == nil {
			msg := "No more work to do"
			logger.Info(msg)
			fmt.Fprintln(w, msg)
			return
		}

		if err = s.exportBatch(ctx, batch); err != nil {
			logger.Errorf("Failed to create files for batch: %v.", err)
			continue
		}

		fmt.Fprintf(w, "Batch %d marked completed. \n", batch.BatchID)
	}
}

func (s *BatchServer) exportBatch(ctx context.Context, eb *model.ExportBatch) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing export batch %d (root: %q, region: %s), max records per file %d", eb.BatchID, eb.FilenameRoot, eb.Region, s.bsc.MaxRecords)

	criteria := database.IterateExposuresCriteria{
		SinceTimestamp:      eb.StartTimestamp,
		UntilTimestamp:      eb.EndTimestamp,
		IncludeRegions:      []string{eb.Region},
		OnlyLocalProvenance: false, // include federated ids
	}

	// Build up groups of exposures in memory. We need to use memory so we can determine the
	// total number of groups (which is embedded in each export file). This technique avoids
	// SELECT COUNT which would lock the database slowing new uploads.
	var groups [][]*model.Exposure
	var exposures []*model.Exposure
	_, err := s.db.IterateExposures(ctx, criteria, func(exp *model.Exposure) error {
		exposures = append(exposures, exp)
		if len(exposures) == s.bsc.MaxRecords {
			groups = append(groups, exposures)
			exposures = nil
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("iterating exposures: %w", err)
	}
	// Create a group for any remaining keys.
	if len(exposures) > 0 {
		groups = append(groups, exposures)
	}

	if len(groups) == 0 {
		logger.Infof("No records for export batch %d", eb.BatchID)
	}

	// Create the export files.
	batchSize := len(groups)
	var objectNames []string
	for i, exposures := range groups {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			logger.Infof("Timed out writing export files for batch %s, the entire batch will be retried once the batch lease expires on %v", eb.BatchID, eb.LeaseExpires)
			return nil
		default:
			// Fallthrough
		}

		// TODO(squee1945): Uploading in parallel (to a point) probably makes better use of network.
		objectName, err := s.createFile(ctx, exposures, eb, i+1, batchSize)
		if err != nil {
			return fmt.Errorf("creating export file %d for batch %d: %w", i+1, eb.BatchID, err)
		}
		logger.Infof("Wrote export file %q for batch %d", objectName, eb.BatchID)
		objectNames = append(objectNames, objectName)
	}

	// Create the index file. The index file includes _all_ batches for an ExportConfig, so multiple
	// workers may be racing to update it. We use a lock to make them line up after one another.
	lockID := fmt.Sprintf("export-batch-%d", eb.BatchID)
	sleep := 10 * time.Second
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			logger.Infof("Timed out acquiring index file lock for batch %s, the entire batch will be retried once the batch lease expires on %v", eb.BatchID, eb.LeaseExpires)
			return nil
		default:
			// Fallthrough
		}

		unlock, err := s.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			if err == database.ErrAlreadyLocked {
				logger.Debugf("Lock %s is locked; sleeping %v and will try again", lockID, sleep)
				time.Sleep(sleep)
				continue
			}
			return err
		}

		indexName, entries, err := s.createIndex(ctx, eb, objectNames)
		if err != nil {
			if err1 := unlock(); err1 != nil {
				return fmt.Errorf("releasing lock: %v (original error: %w)", err1, err)
			}
			return fmt.Errorf("creating index file for batch %d: %w", eb.BatchID, err)
		}
		logger.Infof("Wrote index file %q with %d entries (triggered by batch %d)", indexName, entries, eb.BatchID)
		if err := unlock(); err != nil {
			return fmt.Errorf("releasing lock: %w", err)
		}
		break
	}

	// Write the files records in database and complete the batch.
	if err := s.db.FinalizeBatch(ctx, eb, objectNames, batchSize); err != nil {
		return fmt.Errorf("completing batch: %w", err)
	}
	logger.Infof("Batch %d completed", eb.BatchID)
	return nil
}

func (s *BatchServer) createFile(ctx context.Context, exposures []*model.Exposure, eb *model.ExportBatch, batchNum, batchSize int) (string, error) {
	logger := logging.FromContext(ctx)
	signer, err := s.env.GetSignerForKey(ctx, eb.SigningKey)
	if err != nil {
		return "", fmt.Errorf("unable to get signer for key %v: %w", eb.SigningKey, err)
	}
	// Format keys.
	data, err := MarshalExportFile(eb, exposures, batchNum, batchSize, signer)
	if err != nil {
		return "", fmt.Errorf("marshalling export file: %w", err)
	}

	// Write to GCS.
	objectName := exportFilename(eb, batchNum)
	logger.Infof("Created file %v, signed with key %v", objectName, eb.SigningKey)
	ctx, cancel := context.WithTimeout(ctx, blobOperationTimeout)
	defer cancel()
	if err := s.env.Blobstore.CreateObject(ctx, eb.BucketName, objectName, data); err != nil {
		return "", fmt.Errorf("creating file %s in bucket %s: %w", objectName, eb.BucketName, err)
	}
	return objectName, nil
}

func (s *BatchServer) createIndex(ctx context.Context, eb *model.ExportBatch, newObjectNames []string) (string, int, error) {
	objects, err := s.db.LookupExportFiles(ctx, eb.ConfigID)
	if err != nil {
		return "", 0, fmt.Errorf("lookup existing export files for batch %d: %w", eb.BatchID, err)
	}

	// Add the new objects (they haven't been committed to the database yet).
	objects = append(objects, newObjectNames...)

	// Remove duplicates, sort.
	m := map[string]struct{}{}
	for _, o := range objects {
		m[o] = struct{}{}
	}
	objects = nil
	for k := range m {
		objects = append(objects, k)
	}
	sort.Strings(objects)

	data := []byte(strings.Join(objects, "\n"))

	indexObjectName := exportIndexFilename(eb)
	ctx, cancel := context.WithTimeout(ctx, blobOperationTimeout)
	defer cancel()
	if err := s.env.Blobstore.CreateObject(ctx, eb.BucketName, indexObjectName, data); err != nil {
		return "", 0, fmt.Errorf("creating file %s in bucket %s: %w", indexObjectName, eb.BucketName, err)
	}
	return indexObjectName, len(objects), nil
}

func exportFilename(eb *model.ExportBatch, batchNum int) string {
	return fmt.Sprintf("%s/%d-%05d%s", eb.FilenameRoot, eb.StartTimestamp.Unix(), batchNum, filenameSuffix)
}

func exportIndexFilename(eb *model.ExportBatch) string {
	return fmt.Sprintf("%s/index.txt", eb.FilenameRoot)
}
