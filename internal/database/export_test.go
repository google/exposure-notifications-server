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

package database

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/go-cmp/cmp"
	pgx "github.com/jackc/pgx/v4"
)

func TestAddExportConfig(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	fromTime := time.Now()
	thruTime := fromTime.Add(6 * time.Hour)
	want := &model.ExportConfig{
		BucketName:   "mocked",
		FilenameRoot: "root",
		Period:       3 * time.Hour,
		Region:       "i1",
		From:         fromTime,
		Thru:         thruTime,
		SigningKey:   "key",
	}
	if err := testDB.AddExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}
	conn, err := testDB.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	var (
		got   model.ExportConfig
		psecs int
	)
	err = conn.QueryRow(ctx, `
		SELECT
			config_id, bucket_name, filename_root, period_seconds, region, from_timestamp, thru_timestamp, signing_key
		FROM
			ExportConfig
		WHERE
			config_id = $1
	`, want.ConfigID).Scan(&got.ConfigID, &got.BucketName, &got.FilenameRoot, &psecs, &got.Region, &got.From, &got.Thru, &got.SigningKey)
	if err != nil {
		t.Fatal(err)
	}
	got.Period = time.Duration(psecs) * time.Second

	want.From = want.From.Truncate(time.Microsecond)
	want.Thru = want.Thru.Truncate(time.Microsecond)
	if diff := cmp.Diff(want, &got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestIterateExportConfigs(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	ecs := []*model.ExportConfig{
		{
			BucketName:   "b 1",
			FilenameRoot: "active 1",
			From:         now.Add(-time.Minute),
			Thru:         now.Add(time.Minute),
		},
		{
			BucketName:   "b 2",
			FilenameRoot: "active 2",
			From:         now.Add(-time.Minute),
		},
		{
			BucketName:   "b 3",
			FilenameRoot: "done",
			From:         now.Add(-time.Hour),
			Thru:         now.Add(-time.Minute),
		},
		{
			BucketName:   "b 4",
			FilenameRoot: "not yet",
			From:         now.Add(time.Minute),
			Thru:         now.Add(time.Hour),
		},
	}
	for _, ec := range ecs {
		ec.Period = time.Hour
		ec.Region = "R"
		if err := testDB.AddExportConfig(ctx, ec); err != nil {
			t.Fatal(err)
		}
	}

	var got []*model.ExportConfig
	err := testDB.IterateExportConfigs(ctx, now, func(m *model.ExportConfig) error {
		got = append(got, m)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := ecs[0:2]
	sort.Slice(got, func(i, j int) bool { return got[i].FilenameRoot < got[j].FilenameRoot })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestBatches(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	config := &model.ExportConfig{
		BucketName:   "mocked",
		FilenameRoot: "root",
		Period:       time.Hour,
		Region:       "R",
		From:         now,
		Thru:         now.Add(time.Hour),
	}
	if err := testDB.AddExportConfig(ctx, config); err != nil {
		t.Fatal(err)
	}
	var batches []*model.ExportBatch
	var wantLatest time.Time
	for i := 0; i < 4; i++ {
		start := now.Add(time.Duration(i) * time.Minute)
		end := start.Add(time.Minute)
		wantLatest = end
		batches = append(batches, &model.ExportBatch{
			ConfigID:       config.ConfigID,
			BucketName:     config.BucketName,
			FilenameRoot:   config.FilenameRoot,
			Region:         config.Region,
			Status:         model.ExportBatchOpen,
			StartTimestamp: start,
			EndTimestamp:   end,
		})
	}
	if err := testDB.AddExportBatches(ctx, batches); err != nil {
		t.Fatal(err)
	}

	gotLatest, err := testDB.LatestExportBatchEnd(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	if !gotLatest.Equal(wantLatest) {
		t.Errorf("LatestExportBatchEnd: got %s, want %s", gotLatest, wantLatest)
	}

	leaseBatches := func() int64 {
		t.Helper()
		var batchID int64
		// Lease all the batches.
		for range batches {
			got, err := testDB.LeaseBatch(ctx, time.Hour, now)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("could not lease a batch")
			}
			if got.ConfigID != config.ConfigID || got.FilenameRoot != config.FilenameRoot ||
				got.Region != config.Region || got.BucketName != config.BucketName {
				t.Errorf("LeaseBatch: got (%d, %q, %q, %q), want (%d, %q, %q, %q)",
					got.ConfigID, got.BucketName, got.FilenameRoot, got.Region,
					config.ConfigID, config.BucketName, config.FilenameRoot, config.Region)
			}
			if got.Status != model.ExportBatchPending {
				t.Errorf("LeaseBatch: got status %q, want pending", got.Status)
			}
			wantExpires := now.Add(time.Hour)
			if got.LeaseExpires.Before(wantExpires) || got.LeaseExpires.After(wantExpires.Add(time.Minute)) {
				t.Errorf("LeaseBatch: expires at %s, wanted a time close to %s", got.LeaseExpires, wantExpires)
			}
			batchID = got.BatchID
		}
		// Every batch is leased.
		got, err := testDB.LeaseBatch(ctx, time.Hour, now)
		if got != nil || err != nil {
			t.Errorf("all leased: got (%v, %v), want (nil, nil)", got, err)
		}
		return batchID
	}
	// Now, all end times are in the future, so no batches can be leased.
	got, err := testDB.LeaseBatch(ctx, time.Hour, now)
	if got != nil || err != nil {
		t.Errorf("got (%v, %v), want (nil, nil)", got, err)
	}

	// One hour later, all batches' end times are in the past, so they can be leased.
	now = now.Add(time.Hour)
	leaseBatches()
	// Two hours later all the batches have expired, so we can lease them again.
	now = now.Add(2 * time.Hour)
	batchID := leaseBatches()

	// Complete a batch.
	err = testDB.inTx(ctx, pgx.Serializable, func(tx pgx.Tx) error { return completeBatch(ctx, tx, batchID) })
	if err != nil {
		t.Fatal(err)
	}
	got, err = testDB.LookupExportBatch(ctx, batchID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.ExportBatchComplete {
		t.Errorf("after completion: got status %q, want complete", got.Status)
	}
}

func TestFinalizeBatch(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:   "some-bucket",
		FilenameRoot: "filename-root",
		Period:       time.Minute,
		Region:       "US",
	}
	if err := testDB.AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}

	// Add a batch.
	eb := &model.ExportBatch{
		ConfigID:       ec.ConfigID,
		BucketName:     ec.BucketName,
		FilenameRoot:   ec.FilenameRoot,
		StartTimestamp: now.Add(-2 * time.Hour),
		EndTimestamp:   now.Add(-time.Hour),
		Region:         ec.Region,
		Status:         model.ExportBatchOpen,
	}
	if err := testDB.AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}

	// Lease the batch.
	eb, err := testDB.LeaseBatch(ctx, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the batch is PENDING.
	gotBatch, err := testDB.LookupExportBatch(ctx, eb.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBatch.Status != model.ExportBatchPending {
		t.Errorf("pre gotBatch.Status=%q, want=%q", gotBatch.Status, model.ExportBatchPending)
	}

	// Finalize the batch.
	files := []string{"file1.txt", "file2.txt"}
	batchSize := 10
	if err := testDB.FinalizeBatch(ctx, eb, files, batchSize); err != nil {
		t.Fatal(err)
	}

	// Check that the batch is COMPLETED.
	gotBatch, err = testDB.LookupExportBatch(ctx, eb.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBatch.Status != model.ExportBatchComplete {
		t.Errorf("post gotBatch.Status=%q, want=%q", gotBatch.Status, model.ExportBatchComplete)
	}

	// Check that files were written.
	gotFiles, err := testDB.LookupExportFiles(ctx, eb.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(files, gotFiles); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	for i, filename := range gotFiles {
		got, err := testDB.LookupExportFile(ctx, filename)
		if err != nil {
			t.Fatal(err)
		}
		want := &model.ExportFile{
			BucketName: eb.BucketName,
			Filename:   filename,
			BatchID:    eb.BatchID,
			Region:     eb.Region,
			BatchNum:   i + 1,
			BatchSize:  batchSize,
			Status:     model.ExportBatchComplete,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch for %q (-want, +got):\n%s", filename, diff)
		}
	}
}
