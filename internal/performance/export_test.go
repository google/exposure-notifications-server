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
	"github.com/google/exposure-notifications-server/internal/integration"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/storage"
	testutil "github.com/google/exposure-notifications-server/internal/utils"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/google/exposure-notifications-server/pkg/util"
	pgx "github.com/jackc/pgx/v4"
	"github.com/sethvargo/go-envconfig"
	"github.com/sethvargo/go-retry"
)

type testConfig struct {
	Publishes int  `env:"PUBLISHES,default=1000"`
	Dev       bool `env:"PERFORMANCE_DEV"`
}

func TestExport(t *testing.T) {
	const (
		keysPerPublish = 14
		exportPeriod   = 10 * time.Minute
		totalBatches   = 24 * 6
	)
	var (
		ctx      = context.Background()
		criteria = publishdb.IterateExposuresCriteria{
			OnlyLocalProvenance: false,
		}
		numPublishes = 100000
		// Consider the above publishes are evenly distributed in 24 hours, and
		// period is 10 minutes
		batchStartTime = time.Now().Add(time.Duration(-totalBatches-10) * exportPeriod)
	)
	c := testConfig{}
	if err := envconfig.ProcessWith(context.Background(), &c, envconfig.OsLookuper()); err != nil {
		t.Fatalf("unable to process env: %v", err)
	}

	if c.Dev && c.Publishes > 0 {
		numPublishes = c.Publishes
	}
	want := keysPerPublish * numPublishes
	roughPerBatch := numPublishes/totalBatches + 1

	makoQuickstore, cancel := setup(t)
	defer cancel(context.Background())

	env, client, jwtCfg, exportDir, exportRoot := integration.NewTestServer(t, exportPeriod)
	db := env.Database()
	keys := util.GenerateExposureKeys(keysPerPublish, -1, false)
	payload := &verifyapi.Publish{
		Keys:              keys,
		HealthAuthorityID: "com.example.app",
	}
	jwtCfg.ExposureKeys = keys
	verification, salt := testutil.IssueJWT(t, jwtCfg)
	payload.VerificationPayload = verification
	payload.HMACKey = salt
	if _, err := client.PublishKeys(payload); err != nil {
		t.Fatal(err)
	}

	var exposures []*publishmodel.Exposure
	if _, err := publishdb.New(db).IterateExposures(ctx, criteria, func(m *publishmodel.Exposure) error {
		exposures = append(exposures, m)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if l := len(exposures); l != keysPerPublish {
		t.Fatalf("Want: %d keys, got: %d", keysPerPublish, l)
	}

	// Creates batche based on first batch
	time.Sleep(3 * time.Second)
	if err := client.ExportBatches(); err != nil {
		t.Fatal(err)
	}
	if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
					UPDATE
						ExportBatch
					SET
						start_timestamp = $1,
						end_timestamp = $2
					WHERE
						start_timestamp > $3
				`,
			batchStartTime,
			batchStartTime.Add(exportPeriod),
			time.Now().Add(-2*exportPeriod),
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
	// Delete the first batch of exposures. The creation time of all exposures
	// in this test are modified to fit nicely among exports, delete the first
	// one is easier than running a sql modifying them
	deleted, err := publishdb.New(db).DeleteExposuresBefore(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if deleted != keysPerPublish {
		t.Fatalf("Delete template publishes, want: %d, got: %d", keysPerPublish, deleted)
	}

	// Publish keys based on first batch of published keys
	for i := 0; i < numPublishes; i++ {
		if r := i % roughPerBatch; r == 0 { // increace start time after each batch
			batchStartTime = batchStartTime.Add(exportPeriod)
		}
		var revisedExposures []*publishmodel.Exposure
		for j, newKey := range util.GenerateExposureKeys(keysPerPublish, -1, false) {
			m := *exposures[j]
			m.CreatedAt = batchStartTime.Add(1 * time.Second)
			m.ExposureKey, _ = base64util.DecodeString(newKey.Key)
			revisedExposures = append(revisedExposures, &m)
		}
		updated, err := publishdb.New(db).InsertAndReviseExposures(ctx, revisedExposures,
			nil, false)
		if err != nil {
			t.Fatal(err)
		}
		if updated != keysPerPublish {
			t.Fatalf("Want updated: %d, got %d", keysPerPublish, updated)
		}
	}

	// Start measurement
	startTime := time.Now()
	integration.Eventually(t, 30, func() error {
		// Export batch again to make the rest of batches
		if err := client.ExportBatches(); err != nil {
			t.Fatal(err)
		}

		// Start export workers
		if err := client.StartExportWorkers(); err != nil {
			return err
		}

		index, err := env.Blobstore().GetObject(ctx, exportDir,
			integration.IndexFilePath(exportRoot))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				time.Sleep(500 * time.Millisecond)
				return retry.RetryableError(err)
			}
			return err
		} else if c := strings.TrimSpace(string(index)); c == "" {
			time.Sleep(500 * time.Millisecond)
			return retry.RetryableError(fmt.Errorf("index file %s/%s is empty", exportDir, integration.IndexFilePath(exportRoot)))
		}

		var got int
		for _, f := range strings.Split(string(index), "\n") {
			// Download the latest export file contents
			data, err := env.Blobstore().GetObject(ctx, exportDir, f)
			if err != nil {
				return fmt.Errorf("failed to open %s/%s: %v", exportDir, f, err)
			}

			// Process contents as an export
			key, err := exportapi.UnmarshalExportFile(data)
			if err != nil {
				return fmt.Errorf("failed to extract export data: %v", err)
			}
			got += len(key.Keys)
		}

		if got != want {
			time.Sleep(500 * time.Millisecond)
			return retry.RetryableError(fmt.Errorf("Want exported keys: %d, got: %d", want, got))
		}
		return nil
	})
	exportDuration := time.Now().Sub(startTime)
	t.Logf("Export finished in '%v'", exportDuration)
	if err := makoQuickstore.AddSamplePoint(float64(time.Now().Unix()), map[string]float64{
		"m1": float64(exportDuration / 1000000),
	}); err != nil {
		t.Fatal(err)
	}

	// Next, measure cleanup performance

	// Mark the export in the past to force a cleanup
	if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE
				ExportBatch
			SET
				start_timestamp = $1,
				end_timestamp = $2
		`,
			time.Now().Add(-30*24*time.Hour),
			time.Now().Add(-29*24*time.Hour),
		)
		return err
	}); err != nil {
		t.Fatal(err)
	}

	// 4. Cleanup and capture metrics
	startTime = time.Now()
	if err = client.CleanupExports(); err != nil {
		t.Fatal(err)
	}

	var remainings []string
	integration.Eventually(t, 30, func() error {
		index, err := env.Blobstore().GetObject(ctx, exportDir,
			path.Join(exportRoot, "index.txt"))
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return retry.RetryableError(fmt.Errorf("Can not find index file: %v", err))
			}
			return err
		}
		// Find the latest file in the index
		remainings = strings.Split(string(index), "\n")
		return nil
	})
	for _, r := range remainings {
		_, err := env.Blobstore().GetObject(ctx, exportDir, r)
		if err == nil {
			t.Fatalf("Should have been cleaned up %q: %v", r, err)
		}
		if !errors.Is(err, storage.ErrNotFound) {
			t.Fatalf("Failed reading blob %q: %v", r, err)
		}
	}

	cleanupDuration := time.Now().Sub(startTime)
	t.Logf("Clean up finished in '%v'", cleanupDuration)
	if err := makoQuickstore.AddSamplePoint(float64(time.Now().Unix()), map[string]float64{
		"m2": float64(cleanupDuration / 1000000),
	}); err != nil {
		t.Fatal(err)
	}

	store(t, makoQuickstore)
}
