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
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/integration"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/internal/util"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/sethvargo/go-retry"
)

func TestBatchCleanupBenchmark(t *testing.T) {
	// 1. Publish keys
	const (
		keysPerBatch = 3
		// If consider batching every 10 seconds, 10000 batches is 100000
		// seconds ~= 27 hours worth of exports
		batchCount = 10000
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

	var firstBatchExportFile *exportmodel.ExportFile
	integration.Eventually(t, 30, func() error {
		time.Sleep(2 * time.Second)
		// Trigger an export
		if err := client.ExportBatches(); err != nil {
			return err
		}

		// Start export workers
		if err := client.StartExportWorkers(); err != nil {
			return err
		}

		var firstBatchFiles []string
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
		// Find the latest file in the index
		firstBatchFiles = strings.Split(string(index), "\n")
		if l := len(firstBatchFiles); l != 1 {
			return fmt.Errorf("exported batches. Want: %d, got: %d", 1, l)
		}
		firstBatchExportFile, err = exportdb.New(db).LookupExportFile(ctx, firstBatchFiles[0])
		if err != nil {
			return fmt.Errorf("failed to look up exportfile %q: %w", firstBatchFiles[0], err)
		}
		return nil
	})

	// 1.2 Proliferate exports based on first batch export
	firstBatchExport, err := exportdb.New(db).LookupExportBatch(ctx, firstBatchExportFile.BatchID)
	if err != nil {
		t.Fatalf("Failed looking up the first export: %v", err)
	}
	exportContent, err := env.Blobstore().GetObject(ctx, integration.ExportDir, firstBatchExportFile.Filename)
	if err != nil {
		t.Fatalf("Failed reading exported file %s from %s: %v", firstBatchExportFile.Filename, integration.ExportDir, err)
	}
	createTime := firstBatchExport.StartTimestamp.Add(-366 * time.Hour)
	endTime := firstBatchExport.EndTimestamp.Add(-366 * time.Hour)
	for i := 0; i < batchCount; i++ {
		// For all other batches, copying from first batch
		createTime = createTime.Add(10 * time.Second)
		endTime = endTime.Add(10 * time.Second)
		firstBatchExport.BatchID = firstBatchExport.BatchID + 1
		firstBatchExport.StartTimestamp = createTime
		firstBatchExport.EndTimestamp = endTime
		exportdb.New(db).AddExportBatches(ctx, []*exportmodel.ExportBatch{firstBatchExport})

		fn := fmt.Sprintf("%s/%d-%d.zip", firstBatchExport.FilenameRoot, createTime.Unix(), endTime.Unix())
		// Note: all exported files have identical contents, as:
		// Cleanup exports function works based on what's stored in
		// database, and maps with exportd files only by filenames, so it
		// doesn't really matter what's inside the  exported file.
		if err := env.Blobstore().CreateObject(ctx, integration.ExportDir, fn, exportContent, true); err != nil {
			t.Fatalf("creating file %s in bucket %s: %v", fn, integration.ExportDir, err)
		}
		// Add index file
		contents, err := env.Blobstore().GetObject(ctx, integration.ExportDir, integration.IndexFilePath())
		if err != nil {
			t.Fatalf("Failed reading %s in bucket %s: %v", integration.ExportDir, integration.IndexFilePath(), err)
		}
		contents = []byte(string(contents) + "\n" + fn)
		if err := env.Blobstore().CreateObject(ctx, integration.ExportDir, integration.IndexFilePath(), contents, false); err != nil {
			t.Fatalf("creating file %s in bucket %s: %v", integration.IndexFilePath(), integration.ExportDir, err)
		}
		// FinalizeBatch helps with writing to database table `ExportFile`
		if err := exportdb.New(db).FinalizeBatch(ctx, firstBatchExport, []string{fn}, 1); err != nil {
			t.Fatalf("Failed finalizing new batch: %v", err)
		}
	}

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
		if len(lines) != batchCount+1 {
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

	timeTracker = reportTime(t, timeTracker, "Create workload")

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
		if r == firstBatchExportFile.Filename {
			continue
		}
		_, err := env.Blobstore().GetObject(ctx, integration.ExportDir, r)
		if err == nil {
			t.Fatalf("Should have been cleaned up %q: %v", r, err)
		}
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("Failed reading blob %q: %v", r, err)
		}
	}
}
