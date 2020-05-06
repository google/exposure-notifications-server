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
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/storage"
)

const (
	batchIDParam   = "batch-id"
	filenameSuffix = ".zip"
)

func NewBatchServer(db *database.DB, bsc BatchServerConfig) *BatchServer {
	return &BatchServer{
		db:  db,
		bsc: bsc,
	}
}

// BatchServer hosts end points to manage export batches.
type BatchServer struct {
	db  *database.DB
	bsc BatchServerConfig
}

type BatchServerConfig struct {
	CreateTimeout time.Duration
	TmpBucket     string
	Bucket        string
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
	unlockFn, err := s.db.Lock(ctx, lock, s.bsc.CreateTimeout) // TODO(jasonco): double this?
	if err != nil {
		if errors.Is(err, database.ErrAlreadyLocked) {
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
	it, err := s.db.IterateExportConfigs(ctx, now)
	if err != nil {
		logger.Errorf("Failed to get export config iterator: %v", err)
		http.Error(w, "Failed to get export config iterator, check logs.", http.StatusInternalServerError)
		return
	}
	defer it.Close()

	for {

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

		ec, done, err := it.Next()
		if err != nil {
			logger.Errorf("Failed to iterate export config: %v", err)
			http.Error(w, "Failed to iterate export config, check logs.", http.StatusInternalServerError)
			return
		}
		if done {
			return
		}
		// Iterator may go one past before returning done==true.
		if ec == nil {
			continue
		}

		if err := s.maybeCreateBatches(ctx, ec, now); err != nil {
			logger.Errorf("Failed to create batches for config %d: %v. Continuing", ec.ConfigID, err)
		}
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
		logger.Infof("Batch creation for config %d is not required. Skipping.", ec.ConfigID)
		return nil
	}

	var batches []*model.ExportBatch
	for _, br := range ranges {
		batches = append(batches, &model.ExportBatch{
			ConfigID:       ec.ConfigID,
			FilenameRoot:   ec.FilenameRoot,
			StartTimestamp: br.start,
			EndTimestamp:   br.end,
			Region:         ec.Region,
			Status:         model.ExportBatchOpen,
		})
	}

	if err := s.db.AddExportBatches(ctx, batches); err != nil {
		return fmt.Errorf("creating export batches for config %d: %w", ec.ConfigID, err)
	}

	logger.Infof("Created %d batch(es) for config %d.", len(batches), ec.ConfigID)
	return nil
}

type batchRange struct {
	start, end time.Time
}

var sanityDate = time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC)

func makeBatchRanges(period time.Duration, latestEnd, now time.Time) []batchRange {

	// Truncate now to align with period; use this as the end date.
	end := now.Truncate(period)

	// If the end date < latest end date, we already have a batch that covers this period, so return no batches.
	if end.Before(latestEnd) {
		return nil
	}

	// Subtract period to get the start date.
	start := end.Add(-period)

	// Special case: if there have not been batches before, return only a single one.
	// We use sanityDate here because the loop below will happily create batch ranges
	// until the beginning of time otherwise.
	if latestEnd.Before(sanityDate) {
		return []batchRange{{start: start, end: end}}
	}

	// Build up a list of batches until we reach that latestEnd.
	// Allow for overlap so we don't miss keys; this might happen in the event that
	// an ExportConfig was edited and the new settings don't quite align.
	ranges := []batchRange{}
	for end.After(latestEnd) {
		ranges = append([]batchRange{{start: start, end: end}}, ranges...)
		start = start.Add(-period)
		end = end.Add(-period)
	}
	return ranges
}

// CreateFilesHandler is a handler to iterate the rows of ExportBatch, and creates GCS files
func (s *BatchServer) CreateFilesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	// Poll for a batch and obtain a lease for it
	// TODO(jasonco): Loop while there is work to do?
	ttl := 15 * time.Minute // TODO(jasonco): take from args?
	batch, err := s.db.LeaseBatch(ctx, ttl, time.Now().UTC())
	if err != nil {
		logger.Errorf("Failed to lease batch: %v", err)
		http.Error(w, "Failed to lease batch, check logs.", http.StatusInternalServerError)
		return
	}
	if batch == nil {
		logger.Debugf("No work to do.")
		return
	}

	ctx, cancel := context.WithDeadline(context.Background(), batch.LeaseExpires)
	defer cancel()

	if err = s.exportBatch(ctx, batch); err != nil {
		logger.Errorf("Failed to create files for batch: %v", err)
		http.Error(w, "Failed to create files for batch, check logs.", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Batch %d marked completed", batch.BatchID)
}

func (s *BatchServer) exportBatch(ctx context.Context, eb *model.ExportBatch) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Creating files for export config %#v, max records per file %d", eb, s.bsc.MaxRecords)

	criteria := database.IterateExposuresCriteria{
		SinceTimestamp:      eb.StartTimestamp,
		UntilTimestamp:      eb.EndTimestamp,
		IncludeRegions:      []string{eb.Region},
		OnlyLocalProvenance: false, // include federated ids
	}

	totalExposures, err := s.db.CountExposures(ctx, criteria)
	if err != nil {
		return fmt.Errorf("counting exposures: %w", err)
	}
	batchSize := int(math.Ceil(float64(totalExposures) / float64(s.bsc.MaxRecords)))

	it, err := s.db.IterateExposures(ctx, criteria)
	if err != nil {
		return fmt.Errorf("iterating exposures: %w", err)
	}
	defer it.Close()

	// TODO(lmohanan): Watch for context deadline
	var exposures []*model.Exposure
	var objectNames []string
	for {
		exp, done, err := it.Next()
		if err != nil {
			return err
		}
		if done {
			break
		}
		// Iterator may go one past before returning done==true.
		if exp == nil {
			continue
		}

		exposures = append(exposures, exp)
		if len(exposures) == s.bsc.MaxRecords {
			objectName, err := s.createFile(ctx, exposures, eb, len(objectNames)+1, batchSize)
			if err != nil {
				return err
			}
			objectNames = append(objectNames, objectName)
			exposures = nil
		}
	}
	if err != nil {
		return fmt.Errorf("iterating exposures: %w", err)
	}

	// Create a file for any remaining keys.
	if len(exposures) > 0 {
		objectName, err := s.createFile(ctx, exposures, eb, len(objectNames)+1, batchSize)
		if err != nil {
			return err
		}
		objectNames = append(objectNames, objectName)
	}

	for {
		unlock, err := s.db.Lock(ctx, fmt.Sprintf("export-batch-%d", eb.BatchID), 5*time.Minute)
		if err != nil {
			if err == database.ErrAlreadyLocked {
				time.Sleep(10 * time.Second)
				continue
			}
			return err
		}

		if err := s.createIndex(ctx, eb, objectNames); err != nil {
			if err1 := unlock(); err1 != nil {
				return fmt.Errorf("releasing lock: %v (original error: %w)", err1, err)
			}
			return fmt.Errorf("creating index file: %w", err)
		}
		if err := unlock(); err != nil {
			return fmt.Errorf("releasing lock: %w", err)
		}
		return nil
	}

	if err := s.db.FinalizeBatch(ctx, eb, objectNames, batchSize); err != nil {
		return fmt.Errorf("completing batch: %w", err)
	}
	return nil
}

func (s *BatchServer) createFile(ctx context.Context, exposures []*model.Exposure, eb *model.ExportBatch, batchNum, batchSize int) (string, error) {
	// Format keys
	data, err := MarshalExportFile(eb, exposures, batchNum, batchSize)
	if err != nil {
		return "", fmt.Errorf("marshalling export file: %w", err)
	}

	// Write to GCS
	objectName := fmt.Sprintf("%s/%d-%04d%s", eb.FilenameRoot, eb.StartTimestamp.Unix(), batchNum, filenameSuffix)
	if err := storage.CreateObject(ctx, s.bsc.Bucket, objectName, data); err != nil {
		return "", fmt.Errorf("creating file %s in bucket %s: %w", objectName, s.bsc.Bucket, err)
	}
	return objectName, nil
}

func (s *BatchServer) createIndex(ctx context.Context, eb *model.ExportBatch, newObjectNames []string) error {
	objects, err := s.db.LookupExportFiles(ctx, eb.ConfigID)
	if err != nil {
		return fmt.Errorf("lookup existing export files for batch %d: %w", eb.BatchID, err)
	}

	// Add the new objects (they haven't been committed to the database yet).
	objects = append(objects, newObjectNames...)

	indexObjectName := fmt.Sprintf("%s/index.txt", eb.FilenameRoot)
	data := []byte(strings.Join(objects, "\n"))
	if err := storage.CreateObject(ctx, s.bsc.Bucket, indexObjectName, data); err != nil {
		return fmt.Errorf("creating file %s in bucket %s: %w", indexObjectName, s.bsc.Bucket, err)
	}
	return nil
}

func NewTestExportHandler(db *database.DB) http.Handler {
	return &testExportHandler{db: db}
}

type testExportHandler struct {
	db *database.DB
}

func (h *testExportHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	if err := h.doExport(ctx, limit); err != nil {
		logger.Errorf("test export: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *testExportHandler) doExport(ctx context.Context, limit int) error {
	since := time.Now().UTC().AddDate(0, 0, -5)
	until := time.Now().UTC()
	exposureKeys, err := h.queryExposureKeys(ctx, since, until, limit)
	if err != nil {
		return fmt.Errorf("error getting exposures: %w", err)
	}
	eb := &model.ExportBatch{
		StartTimestamp: since,
		EndTimestamp:   until,
		Region:         "US",
	}
	data, err := MarshalExportFile(eb, exposureKeys, 1, 1)
	if err != nil {
		return fmt.Errorf("error marshalling export file: %w", err)
	}
	objectName := fmt.Sprintf("testExport-%d-records"+filenameSuffix, limit)
	if err := storage.CreateObject(ctx, "apollo-public-bucket", objectName, data); err != nil {
		return fmt.Errorf("error creating cloud storage object: %w", err)
	}
	return nil
}

func (h *testExportHandler) queryExposureKeys(ctx context.Context, since, until time.Time, limit int) ([]*model.Exposure, error) {
	criteria := database.IterateExposuresCriteria{
		SinceTimestamp:      since,
		UntilTimestamp:      until,
		OnlyLocalProvenance: false, // include federated ids
	}
	it, err := h.db.IterateExposures(ctx, criteria)
	if err != nil {
		return nil, err
	}
	defer it.Close()

	var exposures []*model.Exposure
	for {
		exp, done, err := it.Next()
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
		// Iterator may go one past before returning done==true.
		if exp == nil {
			continue
		}
		exposures = append(exposures, exp)
		if len(exposures) == limit {
			break
		}
	}
	return exposures, nil
}
