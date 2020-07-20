// +build performance

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

package performance

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"testing"
	"time"

	exportapi "github.com/google/exposure-notifications-server/internal/export"
	exportdb "github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/integration"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	pgx "github.com/jackc/pgx/v4"
	"github.com/sethvargo/go-retry"
)

func TestBatchCleanupBenchmark(t *testing.T) {
	// 1. Publish keys
	const (
		keysPerBatch = 3
		// If consider batching every 10 seconds, 10000 batches is 100000
		// seconds ~= 27 hours worth of exports
		batchCount = 100
	)
	var (
		err          error
		ctx          = context.Background()
		exportPeriod = 2 * time.Second
		// Tracking time used for each phase
		// TODO: remove this once mako starts collecting metrics
		timeTracker time.Time
		reportTime  = func(t *testing.T, ts time.Time, msg string) time.Time {
			t.Logf("Spent '%d ms' on %q", time.Now().Sub(ts).Milliseconds(), msg)
			return time.Now()
		}
	)

	timeTracker = time.Now()

	// 1.1 Export first batch
	env, client, db := integration.NewTestServer(t, exportPeriod)
	for i := 0; i < batchCount; i++ {
		payload := &verifyapi.Publish{
			Keys:           util.GenerateExposureKeys(3, -1, false),
			Regions:        []string{"TEST"},
			AppPackageName: "com.example.app",

			// TODO: hook up verification
			VerificationPayload: "TODO",
		}
		if err = client.PublishKeys(payload); err != nil {
			t.Fatal(err)
		}

		if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			result, err := tx.Exec(ctx, `
				WITH cte AS (
					SELECT exposure_key
					FROM Exposure
					LIMIT 1
				)
	
				UPDATE
					Exposure e
				SET
					created_at = $1
				FROM cte
				WHERE e.exposure_key = cte.exposure_key
			`,
				time.Now().Add(time.Duration(i-batchCount)*2*time.Second),
			)
			if err != nil {
				return err
			}
			if got, want := result.RowsAffected(), int64(1); got != want {
				return fmt.Errorf("expected %v to be %v", got, want)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}

		// var firstBatchExportFile *exportmodel.ExportFile
		integration.Eventually(t, 30, func() error {
			// Trigger an export
			if err := client.ExportBatches(); err != nil {
				return err
			}

			// Start export workers
			if err := client.StartExportWorkers(); err != nil {
				return err
			}

			// var firstBatchFiles []string
			index, err := env.Blobstore().GetObject(ctx, integration.ExportDir,
				path.Join(integration.FileNameRoot, "index.txt"))
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return retry.RetryableError(err)
				}
				return err
			} else if c := strings.TrimSpace(string(index)); c == "" {
				return retry.RetryableError(errors.New("index file is empty"))
			}
			return nil
		})

		var batchFiles []string
		integration.Eventually(t, 30, func() error {
			// Trigger an export
			if err := client.ExportBatches(); err != nil {
				return err
			}

			// Start export workers
			if err := client.StartExportWorkers(); err != nil {
				return err
			}

			// Attempt to get the index
			index, err := env.Blobstore().GetObject(ctx, integration.ExportDir, integration.IndexFilePath())
			if err != nil {
				if errors.Is(err, storage.ErrNotFound) {
					return retry.RetryableError(err)
				}
				return err
			}

			// Ensure the new export is created
			batchFiles = strings.Split(string(index), "\n")
			if len(batchFiles) != i+1 {
				return retry.RetryableError(fmt.Errorf("next export does not exist yet"))
			}
			return nil
		})

		// Find the export file that contains the batch - we need this to modify the
		// batch later to force it to cleanup.
		exportFile, err := exportdb.New(db).LookupExportFile(ctx, batchFiles[0])
		if err != nil {
			t.Fatal(err)
		}

		// Mark the export in the past to force a cleanup
		if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
			result, err := tx.Exec(ctx, `
			UPDATE
				ExportBatch
			SET
				start_timestamp = $1,
				end_timestamp = $2
			WHERE
				batch_id = $3
		`,
				time.Now().Add(time.Duration(i-batchCount)*2*time.Second),
				time.Now().Add(time.Duration(i-batchCount)*2*time.Second),
				exportFile.BatchID,
			)
			if err != nil {
				return err
			}
			if got, want := result.RowsAffected(), int64(1); got != want {
				return fmt.Errorf("expected %v to be %v", got, want)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	timeTracker = reportTime(t, timeTracker, "Create workload")

	// 2. Ensure all keys are there
	integration.Eventually(t, 30, func() error {
		index, err := env.Blobstore().GetObject(ctx, integration.ExportDir,
			path.Join(integration.FileNameRoot, "index.txt"))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(fmt.Errorf("Can not find index file %q: %v", integration.IndexFilePath(), err))
			}
			return err
		}
		// Find the latest file in the index
		lines := strings.Split(string(index), "\n")
		if len(lines) != batchCount {
			return fmt.Errorf("exported batches. Want: %d, got: %d", batchCount, len(lines))
		}
		for _, f := range lines {
			data, err := env.Blobstore().GetObject(ctx, integration.ExportDir, f)
			if err != nil {
				return fmt.Errorf("failed to open %s/%s: %w", integration.ExportDir, f, err)
			}

			// Process contents as an export
			key, err := exportapi.UnmarshalExportFile(data)
			if err != nil {
				return fmt.Errorf("failed to extract export data: %w", err)
			}
			if l := len(key.Keys); l != keysPerBatch {
				return fmt.Errorf("exported keys. Want: %d keys. Got: %d keys", keysPerBatch, l)
			}
		}
		return nil
	})

	timeTracker = reportTime(t, timeTracker, "Checking workload")

	// [TODO]3. Set up mako

	// 4. Cleanup and capture metrics
	if err = client.CleanupExports(); err != nil {
		t.Fatal(err)
	}
	timeTracker = reportTime(t, timeTracker, "Invoke Cleanup")

	var remainings []string
	integration.Eventually(t, 30, func() error {
		index, err := env.Blobstore().GetObject(ctx, integration.ExportDir,
			path.Join(integration.FileNameRoot, "index.txt"))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(fmt.Errorf("Can not find index file: %v", err))
			}
			return err
		}
		// Find the latest file in the index
		remainings := strings.Split(string(index), "\n")
		if l := len(remainings); l != batchCount+1 {
			return fmt.Errorf("exported batches. Want: %d, got: %d", batchCount, l)
		}
		return nil
	})
	for _, r := range remainings {
		_, err := env.Blobstore().GetObject(ctx, integration.ExportDir, r)
		if err == nil {
			t.Fatalf("Should have been cleaned up %q: %v", r, err)
		}
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("Failed reading blob %q: %v", r, err)
		}
	}
}
