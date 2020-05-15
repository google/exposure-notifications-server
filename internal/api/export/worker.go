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
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
)

const (
	filenameSuffix       = ".zip"
	blobOperationTimeout = 50 * time.Second
)

// WorkerHandler is a handler to iterate the rows of ExportBatch, and creates GCS files.
func (s *Server) WorkerHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.WorkerTimeout)
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

		// Only consider batches that closed a few minutes ago to allow the publish windows to close properly.
		minutesAgo := time.Now().Add(-5 * time.Minute)

		// Check for a batch and obtain a lease for it.
		batch, err := s.db.LeaseBatch(ctx, s.config.WorkerTimeout, minutesAgo)
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

func (s *Server) exportBatch(ctx context.Context, eb *model.ExportBatch) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing export batch %d (root: %q, region: %s), max records per file %d", eb.BatchID, eb.FilenameRoot, eb.Region, s.config.MaxRecords)

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
		if len(exposures) == s.config.MaxRecords {
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

func (s *Server) createFile(ctx context.Context, exposures []*model.Exposure, eb *model.ExportBatch, batchNum, batchSize int) (string, error) {
	logger := logging.FromContext(ctx)
	signer, err := s.env.GetSignerForKey(ctx, eb.SigningKey)
	if err != nil {
		return "", fmt.Errorf("unable to get signer for key %v: %w", eb.SigningKey, err)
	}
	// Generate exposure key export file.
	data, err := MarshalExportFile(eb, exposures, batchNum, batchSize, signer, s.config.DefaultKeyID)
	if err != nil {
		return "", fmt.Errorf("marshalling export file: %w", err)
	}

	// Write to GCS.
	objectName := exportFilename(eb, batchNum)
	logger.Infof("Created file %v, signed with key %v", objectName, eb.SigningKey)
	ctx, cancel := context.WithTimeout(ctx, blobOperationTimeout)
	defer cancel()
	if err := s.env.Blobstore().CreateObject(ctx, eb.BucketName, objectName, data); err != nil {
		return "", fmt.Errorf("creating file %s in bucket %s: %w", objectName, eb.BucketName, err)
	}
	return objectName, nil
}

func (s *Server) createIndex(ctx context.Context, eb *model.ExportBatch, newObjectNames []string) (string, int, error) {
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
	if err := s.env.Blobstore().CreateObject(ctx, eb.BucketName, indexObjectName, data); err != nil {
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
