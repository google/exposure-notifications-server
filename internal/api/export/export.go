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
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/storage"
)

const (
	filenameSuffix = ".zip"
)

// NewBatchServer makes a BatchServer.
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

// BatchServerConfig is configuratiion for a BatchServer.
type BatchServerConfig struct {
	CreateTimeout time.Duration
	WorkerTimeout time.Duration
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
	unlockFn, err := s.db.Lock(ctx, lock, s.bsc.CreateTimeout)
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
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				logger.Errorf("Context error: %v", err)
				return
			}
			logger.Infof("Timed out creating batches. Batch creation will continue on next invocation.")
			return
		default:
			// Fallthrough
		}

		ec, done, err := it.Next()
		if err != nil {
			logger.Errorf("Failed to iterate export config: %v", err)
			http.Error(w, "Failed to iterate export config, check logs.", http.StatusInternalServerError)
			return
		}
		if ec == nil {
			// Iterator may go one past before returning done==true.
			if done {
				return
			}
			continue
		}

		if err := s.maybeCreateBatches(ctx, ec, now); err != nil {
			logger.Errorf("Failed to create batches for config %d: %v. Continuing to next config.", ec.ConfigID, err)
		}
		if done {
			return
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
		batch, err := s.db.LeaseBatch(ctx, s.bsc.WorkerTimeout, time.Now().UTC())
		if err != nil {
			logger.Errorf("Failed to lease batch: %v", err)
			continue
		}
		if batch == nil {
			msg := "No work to do."
			logger.Debugf(msg)
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
	logger.Infof("Creating files for export config %#v, max records per file %d.", eb, s.bsc.MaxRecords)

	criteria := database.IterateExposuresCriteria{
		SinceTimestamp:      eb.StartTimestamp,
		UntilTimestamp:      eb.EndTimestamp,
		IncludeRegions:      []string{eb.Region},
		OnlyLocalProvenance: false, // include federated ids
	}

	it, err := s.db.IterateExposures(ctx, criteria)
	if err != nil {
		return fmt.Errorf("iterating exposures: %w", err)
	}
	defer it.Close()

	// Build up groups of exposures in memory. We need to use memory so we can determine the
	// total number of groups (which is embedded in each export file). This technique avoids
	// SELECT COUNT which would lock the database slowing new uploads.
	var groups [][]*model.Exposure
	var exposures []*model.Exposure
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.DeadlineExceeded && err != context.Canceled {
				return err
			}
			logger.Infof("Timed out iterating exposures for batch %s. The entire batch will be retried once the batch lease expires on %v.", eb.BatchID, eb.LeaseExpires)
			return nil
		default:
			// Fallthrough
		}

		exp, done, err := it.Next()
		if err != nil {
			return err
		}
		if exp == nil {
			// Iterator may go one past before returning done==true.
			if done {
				break
			}
			continue
		}

		exposures = append(exposures, exp)
		if len(exposures) == s.bsc.MaxRecords {
			groups = append(groups, exposures)
			exposures = nil
		}
		if done {
			break
		}
	}

	// Create a group for any remaining keys.
	if len(exposures) > 0 {
		groups = append(groups, exposures)
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
			logger.Infof("Timed out writing export files for batch %s. The entire batch will be retried once the batch lease expires on %v.", eb.BatchID, eb.LeaseExpires)
			return nil
		default:
			// Fallthrough
		}

		// TODO(jasonco): Uploading in parallel (to a point) probably makes better use of network.
		objectName, err := s.createFile(ctx, exposures, eb, i+1, batchSize)
		if err != nil {
			return fmt.Errorf("creating export file %d for batch %d: %w", i+1, eb.BatchID, err)
		}
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
			logger.Infof("Timed out acquiring index file lock for batch %s. The entire batch will be retried once the batch lease expires on %v.", eb.BatchID, eb.LeaseExpires)
			return nil
		default:
			// Fallthrough
		}

		unlock, err := s.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			if err == database.ErrAlreadyLocked {
				logger.Debugf("Lock %s is locked; sleeping %v and will try again.", lockID, sleep)
				time.Sleep(sleep)
				continue
			}
			return err
		}

		if err := s.createIndex(ctx, eb, objectNames); err != nil {
			if err1 := unlock(); err1 != nil {
				return fmt.Errorf("releasing lock: %v (original error: %w)", err1, err)
			}
			return fmt.Errorf("creating index file for batch %d: %w", eb.BatchID, err)
		}
		if err := unlock(); err != nil {
			return fmt.Errorf("releasing lock: %w", err)
		}
		return nil
	}

	// Write the files records in database and complete the batch.
	if err := s.db.FinalizeBatch(ctx, eb, objectNames, batchSize); err != nil {
		return fmt.Errorf("completing batch: %w", err)
	}
	logger.Infof("Batch %d completed.", eb.BatchID)
	return nil
}

func (s *BatchServer) createFile(ctx context.Context, exposures []*model.Exposure, eb *model.ExportBatch, batchNum, batchSize int) (string, error) {
	// Format keys.
	data, err := MarshalExportFile(eb, exposures, batchNum, batchSize)
	if err != nil {
		return "", fmt.Errorf("marshalling export file: %w", err)
	}

	// Write to GCS.
	objectName := exportFilename(eb, batchNum)
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
	if err := storage.CreateObject(ctx, s.bsc.Bucket, indexObjectName, data); err != nil {
		return fmt.Errorf("creating file %s in bucket %s: %w", indexObjectName, s.bsc.Bucket, err)
	}
	return nil
}

func exportFilename(eb *model.ExportBatch, batchNum int) string {
	return fmt.Sprintf("%s/%d-%05d%s", eb.FilenameRoot, eb.StartTimestamp.Unix(), batchNum, filenameSuffix)
}

func exportIndexFilename(eb *model.ExportBatch) string {
	return fmt.Sprintf("%s/index.txt", eb.FilenameRoot)
}

// NewTestExportHandler is a test handler to write a key file in GCS.
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
		if exp == nil {
			// Iterator may go one past before returning done==true.
			if done {
				break
			}
			continue
		}
		exposures = append(exposures, exp)
		if len(exposures) == limit {
			break
		}
		if done {
			break
		}
	}
	return exposures, nil
}
