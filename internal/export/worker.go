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
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export/database"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"

	"github.com/google/exposure-notifications-server/internal/export/model"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/util"
)

const (
	filenameSuffix       = ".zip"
	blobOperationTimeout = 50 * time.Second
)

// WorkerHandler is a handler to iterate the rows of ExportBatch, and creates export files.
func (s *Server) WorkerHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.config.WorkerTimeout)
	defer cancel()
	logger := logging.FromContext(ctx)
	exportDB := database.New(s.db)

	emitIndexForEmptyBatch := true
	for {
		if ctx.Err() != nil {
			msg := "Timed out processing batches. Will continue on next invocation."
			logger.Info(msg)
			fmt.Fprintln(w, msg)
			return
		}

		// Only consider batches that closed a few minutes ago to allow the publish windows to close properly.
		minutesAgo := time.Now().Add(-5 * time.Minute)

		// Check for a batch and obtain a lease for it.
		batch, err := exportDB.LeaseBatch(ctx, s.config.WorkerTimeout, minutesAgo)
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

		if err = s.exportBatch(ctx, batch, emitIndexForEmptyBatch); err != nil {
			logger.Errorf("Failed to create files for batch: %v.", err)
			continue
		}
		// We re-write the index file for empty batches for self-healing so that the index
		// file reflects the ExportFile table in database. However, if a single worker
		// processes a number of empty batches quickly, we want to avoid writing the same
		// file repeatedly and hitting a rate limit.
		emitIndexForEmptyBatch = false

		fmt.Fprintf(w, "Batch %d marked completed. \n", batch.BatchID)
	}
}

func (s *Server) exportBatch(ctx context.Context, eb *model.ExportBatch, emitIndexForEmptyBatch bool) error {
	logger := logging.FromContext(ctx)
	logger.Infof("Processing export batch %d (root: %q, region: %s), max records per file %d", eb.BatchID, eb.FilenameRoot, eb.Region, s.config.MaxRecords)

	criteria := publishdb.IterateExposuresCriteria{
		SinceTimestamp:      eb.StartTimestamp,
		UntilTimestamp:      eb.EndTimestamp,
		IncludeRegions:      []string{eb.Region},
		OnlyLocalProvenance: false, // include federated ids
	}

	// Build up groups of exposures in memory. We need to use memory so we can determine the
	// total number of groups (which is embedded in each export file). This technique avoids
	// SELECT COUNT which would lock the database slowing new uploads.
	var groups [][]*publishmodel.Exposure
	var exposures []*publishmodel.Exposure

	_, err := s.publishdb.IterateExposures(ctx, criteria, func(exp *publishmodel.Exposure) error {
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

	exposures, err = ensureMinNumExposures(exposures, eb.Region, s.config.MinRecords, s.config.PaddingRange)
	if err != nil {
		return fmt.Errorf("ensureMinNumExposures: %w", err)
	}

	// Load the non-expired signature infos associated with this export batch.
	sigInfos, err := s.exportdb.LookupSignatureInfos(ctx, eb.SignatureInfoIDs, time.Now())
	if err != nil {
		return fmt.Errorf("error loading signature info for batch %d, %w", eb.BatchID, err)
	}

	// Create the export files.
	batchSize := len(groups)
	var objectNames []string
	for i, exposures := range groups {
		if ctx.Err() != nil {
			logger.Infof("Timed out writing export files for batch %s, the entire batch will be retried once the batch lease expires on %v", eb.BatchID, eb.LeaseExpires)
			return nil
		}

		// TODO(squee1945): Uploading in parallel (to a point) probably makes better use of network.
		objectName, err := s.createFile(ctx,
			createFileInfo{
				exposures:      exposures,
				exportBatch:    eb,
				signatureInfos: sigInfos,
				batchNum:       i + 1,
				batchSize:      batchSize,
			})
		if err != nil {
			return fmt.Errorf("creating export file %d for batch %d: %w", i+1, eb.BatchID, err)
		}
		logger.Infof("Wrote export file %q for batch %d", objectName, eb.BatchID)
		objectNames = append(objectNames, objectName)
	}

	// Emit the index file if needed.
	if batchSize > 0 || emitIndexForEmptyBatch {
		if err := s.retryingCreateIndex(ctx, eb, objectNames); err != nil {
			return err
		}
	}

	// Write the files records in database and complete the batch.
	if err := s.exportdb.FinalizeBatch(ctx, eb, objectNames, batchSize); err != nil {
		return fmt.Errorf("completing batch: %w", err)
	}
	logger.Infof("Batch %d completed", eb.BatchID)
	return nil
}

type createFileInfo struct {
	exposures      []*publishmodel.Exposure
	exportBatch    *model.ExportBatch
	signatureInfos []*model.SignatureInfo
	batchNum       int
	batchSize      int
}

func (s *Server) createFile(ctx context.Context, cfi createFileInfo) (string, error) {
	logger := logging.FromContext(ctx)

	var signers []ExportSigners
	for _, si := range cfi.signatureInfos {
		signer, err := s.env.GetSignerForKey(ctx, si.SigningKey)
		if err != nil {
			return "", fmt.Errorf("unable to get signer for key %v: %w", si.SigningKey, err)
		}
		signers = append(signers, ExportSigners{SignatureInfo: si, Signer: signer})
	}

	// Generate exposure key export file.
	data, err := MarshalExportFile(cfi.exportBatch, cfi.exposures, cfi.batchNum, cfi.batchSize, signers)
	if err != nil {
		return "", fmt.Errorf("marshalling export file: %w", err)
	}

	objectName := exportFilename(cfi.exportBatch, cfi.batchNum)
	logger.Infof("Created file %v, signed with %v keys", objectName, len(signers))
	ctx, cancel := context.WithTimeout(ctx, blobOperationTimeout)
	defer cancel()
	if err := s.env.Blobstore().CreateObject(ctx, cfi.exportBatch.BucketName, objectName, data); err != nil {
		return "", fmt.Errorf("creating file %s in bucket %s: %w", objectName, cfi.exportBatch.BucketName, err)
	}
	return objectName, nil
}

// retryingCreateIndex create the index file. The index file includes _all_ batches for an ExportConfig,
// so multiple workers may be racing to update it. We use a lock to make them line up after one another.
func (s *Server) retryingCreateIndex(ctx context.Context, eb *model.ExportBatch, objectNames []string) error {
	logger := logging.FromContext(ctx)

	lockID := fmt.Sprintf("export-batch-%d", eb.BatchID)
	sleep := 10 * time.Second
	for {
		if ctx.Err() != nil {
			logger.Infof("Timed out acquiring index file lock for batch %s, the entire batch will be retried once the batch lease expires on %v", eb.BatchID, eb.LeaseExpires)
			return nil
		}

		unlock, err := s.db.Lock(ctx, lockID, time.Minute)
		if err != nil {
			if err == coredb.ErrAlreadyLocked {
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
	return nil
}

func (s *Server) createIndex(ctx context.Context, eb *model.ExportBatch, newObjectNames []string) (string, int, error) {
	exportDB := database.New(s.db)
	objects, err := exportDB.LookupExportFiles(ctx, eb.ConfigID)
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

// randomInt is inclusive, [min:max]
func randomInt(min, max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()) + min, nil
}

func ensureMinNumExposures(exposures []*publishmodel.Exposure, region string, minLength, jitter int) ([]*publishmodel.Exposure, error) {
	if len(exposures) == 0 {
		return exposures, nil
	}

	extra, _ := randomInt(0, jitter)
	target := minLength + extra

	for len(exposures) < target {
		// Pieces needed are
		// (1) exposure key, (2) interval number, (3) transmission risk
		// Exposure key is 16 random bytes.
		eKey := make([]byte, publishmodel.KeyLength)
		_, err := rand.Read(eKey)
		if err != nil {
			return nil, fmt.Errorf("rand.Read: %w", err)
		}

		// Transmission risk is within the bounds.
		transmissionRisk, err := util.RandomTransmissionRisk()
		if err != nil {
			return nil, fmt.Errorf("randomInt: %w", err)
		}

		// The interval number is pulled from an existing one in the batch
		// at random.
		fromIdx, err := randomInt(0, len(exposures)-1)
		if err != nil {
			return nil, fmt.Errorf("randomInt: %w", err)
		}
		intervalNumber := exposures[fromIdx].IntervalNumber
		// Same with the interval count.
		fromIdx, err = randomInt(0, len(exposures)-1)
		if err != nil {
			return nil, fmt.Errorf("randomInt: %w", err)
		}
		intervalCount := exposures[fromIdx].IntervalCount

		ek := &publishmodel.Exposure{
			ExposureKey:      eKey,
			TransmissionRisk: transmissionRisk,
			Regions:          []string{region},
			IntervalNumber:   intervalNumber,
			IntervalCount:    intervalCount,
			// The rest of the publishmodel.Exposure fields are not used in the export file.
		}
		exposures = append(exposures, ek)
	}

	return exposures, nil
}
