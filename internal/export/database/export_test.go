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
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/go-cmp/cmp"
	pgx "github.com/jackc/pgx/v4"
)

func TestAddRetrieveUpdateSignatureInfo(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	want := &model.SignatureInfo{
		SigningKey:        "/kms/project/key/1",
		SigningKeyVersion: "1",
		SigningKeyID:      "310",
		EndTimestamp:      time.Time{},
	}
	exDB := New(testDB)
	if err := exDB.AddSignatureInfo(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := exDB.GetSignatureInfo(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	// Update, set expiry timestamp.
	want.EndTimestamp = time.Now().UTC().Add(24 * time.Hour)
	want.EndTimestamp = want.EndTimestamp.Truncate(time.Second)
	if err := exDB.UpdateSignatureInfo(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err = exDB.GetSignatureInfo(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}

	got.EndTimestamp = got.EndTimestamp.Truncate(time.Second)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestLookupSignatureInfos(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	testTime := time.Now().UTC()
	want := []*model.SignatureInfo{
		{
			SigningKey:        "/kms/project/key/version/1",
			SigningKeyVersion: "1",
			SigningKeyID:      "310",
			EndTimestamp:      testTime.Add(-1 * time.Hour).Truncate(time.Microsecond),
		},
		{
			SigningKey:        "/kms/project/key/version/2",
			SigningKeyVersion: "2",
			SigningKeyID:      "310",
			EndTimestamp:      testTime.Add(24 * time.Hour).Truncate(time.Microsecond),
		},
		{
			SigningKey:        "/kms/project/key/version/3",
			SigningKeyVersion: "3",
			SigningKeyID:      "310",
		},
	}
	for _, si := range want {
		New(testDB).AddSignatureInfo(ctx, si)
	}

	ids := []int64{want[0].ID, want[1].ID, want[2].ID}
	got, err := New(testDB).LookupSignatureInfos(ctx, ids, testTime)
	if err != nil {
		t.Fatal(err)
	}

	// The first entry (want[0]) is expired and won't be returned.
	want = want[1:]

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%v", diff)
	}
}

func TestAddGetUpdateExportConfig(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	exportDB := New(testDB)

	fromTime := time.Now()
	thruTime := fromTime.Add(6 * time.Hour)
	want := &model.ExportConfig{
		BucketName:       "mocked",
		FilenameRoot:     "root",
		Period:           3 * time.Hour,
		Region:           "i1",
		From:             fromTime,
		Thru:             thruTime,
		SignatureInfoIDs: []int64{42, 84},
	}
	if err := exportDB.AddExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := exportDB.GetExportConfig(ctx, want.ConfigID)
	if err != nil {
		t.Fatal(err)
	}

	want.From = want.From.Truncate(time.Microsecond)
	want.Thru = want.Thru.Truncate(time.Microsecond)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Now update it.
	want.Period = 15 * time.Minute
	want.Thru = time.Time{}
	want.SignatureInfoIDs = []int64{1, 2, 3, 4, 5}

	if err := exportDB.UpdateExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err = exportDB.GetExportConfig(ctx, want.ConfigID)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestIterateExportConfigs(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
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
		if err := New(testDB).AddExportConfig(ctx, ec); err != nil {
			t.Fatal(err)
		}
	}

	var got []*model.ExportConfig
	err := New(testDB).IterateExportConfigs(ctx, now, func(m *model.ExportConfig) error {
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
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	now := time.Now().Truncate(time.Microsecond)
	config := &model.ExportConfig{
		BucketName:       "mocked",
		FilenameRoot:     "root",
		Period:           time.Hour,
		Region:           "R",
		From:             now,
		Thru:             now.Add(time.Hour),
		SignatureInfoIDs: []int64{1, 2, 3, 4},
	}
	if err := New(testDB).AddExportConfig(ctx, config); err != nil {
		t.Fatal(err)
	}
	var batches []*model.ExportBatch
	var wantLatest time.Time
	for i := 0; i < 4; i++ {
		start := now.Add(time.Duration(i) * time.Minute)
		end := start.Add(time.Minute)
		wantLatest = end
		batches = append(batches, &model.ExportBatch{
			ConfigID:         config.ConfigID,
			BucketName:       config.BucketName,
			FilenameRoot:     config.FilenameRoot,
			Region:           config.Region,
			Status:           model.ExportBatchOpen,
			StartTimestamp:   start,
			EndTimestamp:     end,
			SignatureInfoIDs: []int64{1, 2, 3, 4},
		})
	}
	if err := New(testDB).AddExportBatches(ctx, batches); err != nil {
		t.Fatal(err)
	}

	gotLatest, err := New(testDB).LatestExportBatchEnd(ctx, config)
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
			got, err := New(testDB).LeaseBatch(ctx, time.Hour, now)
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
		got, err := New(testDB).LeaseBatch(ctx, time.Hour, now)
		if got != nil || err != nil {
			t.Errorf("all leased: got (%v, %v), want (nil, nil)", got, err)
		}
		return batchID
	}
	// Now, all end times are in the future, so no batches can be leased.
	got, err := New(testDB).LeaseBatch(ctx, time.Hour, now)
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
	err = testDB.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error { return completeBatch(ctx, tx, batchID) })
	if err != nil {
		t.Fatal(err)
	}
	got, err = New(testDB).LookupExportBatch(ctx, batchID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.ExportBatchComplete {
		t.Errorf("after completion: got status %q, want complete", got.Status)
	}
}

func TestFinalizeBatch(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	exportDB := New(testDB)
	ctx := context.Background()
	now := time.Now().Truncate(time.Microsecond)

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:   "some-bucket",
		FilenameRoot: "filename-root",
		Period:       time.Minute,
		Region:       "US",
	}
	if err := exportDB.AddExportConfig(ctx, ec); err != nil {
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
	if err := exportDB.AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}

	// Lease the batch.
	eb, err := exportDB.LeaseBatch(ctx, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the batch is PENDING.
	gotBatch, err := exportDB.LookupExportBatch(ctx, eb.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBatch.Status != model.ExportBatchPending {
		t.Errorf("pre gotBatch.Status=%q, want=%q", gotBatch.Status, model.ExportBatchPending)
	}

	// Finalize the batch.
	files := []string{"file1.txt", "file2.txt"}
	batchSize := 10
	if err := exportDB.FinalizeBatch(ctx, eb, files, batchSize); err != nil {
		t.Fatal(err)
	}

	// Check that the batch is COMPLETED.
	gotBatch, err = exportDB.LookupExportBatch(ctx, eb.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if gotBatch.Status != model.ExportBatchComplete {
		t.Errorf("post gotBatch.Status=%q, want=%q", gotBatch.Status, model.ExportBatchComplete)
	}

	// Check that files were written.
	gotFiles, err := exportDB.LookupExportFiles(ctx, eb.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(files, gotFiles); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	for i, filename := range gotFiles {
		got, err := exportDB.LookupExportFile(ctx, filename)
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

// TestKeysInBatch ensures that keys are fetched in the correct batch when they fall on boundary conditions.
func TestKeysInBatch(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	now := time.Now()

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:   "bucket-name",
		FilenameRoot: "filename-root",
		Period:       3600 * time.Second,
		Region:       "US",
		From:         now.Add(-24 * time.Hour),
	}
	if err := New(testDB).AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}

	// Create a batch for two hours ago to one hour ago.
	startTimestamp := now.Truncate(time.Hour).Add(-2 * time.Hour)
	endTimestamp := startTimestamp.Add(time.Hour)
	eb := &model.ExportBatch{
		ConfigID:       ec.ConfigID,
		BucketName:     ec.BucketName,
		FilenameRoot:   ec.FilenameRoot,
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		Region:         ec.Region,
		Status:         model.ExportBatchOpen,
	}
	if err := New(testDB).AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}

	// Create key aligned with the StartTimestamp
	sek := &publishmodel.Exposure{
		ExposureKey: []byte("aaa"),
		Regions:     []string{ec.Region},
		CreatedAt:   startTimestamp,
	}

	// Create key aligned with the EndTimestamp
	eek := &publishmodel.Exposure{
		ExposureKey: []byte("bbb"),
		Regions:     []string{ec.Region},
		CreatedAt:   endTimestamp,
	}

	// Add the keys to the database.
	if err := publishdb.New(testDB).InsertExposures(ctx, []*publishmodel.Exposure{sek, eek}); err != nil {
		t.Fatal(err)
	}

	// Re-fetch the ExposureBatch by leasing it; this is important to this test which is trying
	// to ensure our dates are going in-and-out of the database correctly.
	leased, err := New(testDB).LeaseBatch(ctx, time.Hour, now)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the batch times from the database are *exactly* what we started with.
	if eb.StartTimestamp.UnixNano() != leased.StartTimestamp.UnixNano() {
		t.Errorf("Start timestamps did not align original: %d, leased: %d", eb.StartTimestamp.UnixNano(), leased.StartTimestamp.UnixNano())
	}
	if eb.EndTimestamp.UnixNano() != leased.EndTimestamp.UnixNano() {
		t.Errorf("End timestamps did not align original: %d, leased: %d", eb.EndTimestamp.UnixNano(), leased.EndTimestamp.UnixNano())
	}

	// Lookup the keys; they must be only the key created_at the startTimestamp
	// (because start is inclusive, end is exclusive).
	criteria := publishdb.IterateExposuresCriteria{
		IncludeRegions: []string{leased.Region},
		SinceTimestamp: leased.StartTimestamp,
		UntilTimestamp: leased.EndTimestamp,
	}

	var got []*publishmodel.Exposure
	_, err = publishdb.New(testDB).IterateExposures(ctx, criteria, func(exp *publishmodel.Exposure) error {
		got = append(got, exp)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 1 {
		t.Fatalf("Incorrect exposure key result length, got %d, want 1", len(got))
	}
	want := []byte("aaa")
	if string(got[0].ExposureKey) != string(want) {
		t.Fatalf("Incorrect exposure key in batch, got %q, want %q", got[0].ExposureKey, want)
	}
}

// TestAddExportFileSkipsDuplicates ensures that ExportFile records are not overwritten.
func TestAddExportFileSkipsDuplicates(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	exportDB := New(testDB)
	ctx := context.Background()

	// Add foreign key records.
	ec := &model.ExportConfig{Period: time.Hour}
	if err := exportDB.AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}
	eb := &model.ExportBatch{ConfigID: ec.ConfigID, Status: model.ExportBatchOpen}
	if err := exportDB.AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}
	// Lease the batch to get the ID.
	eb, err := exportDB.LeaseBatch(ctx, time.Hour, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	wantBucketName := "bucket-1"
	ef := &model.ExportFile{
		Filename:   "file",
		BucketName: wantBucketName,
		BatchID:    eb.BatchID,
	}

	// Add a record.
	err = testDB.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		if err := addExportFile(ctx, tx, ef); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Check that the row is present.
	got, err := exportDB.LookupExportFile(ctx, ef.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if got.BucketName != wantBucketName {
		t.Fatalf("bucket name mismatch got %q, want %q", got.BucketName, wantBucketName)
	}

	// Add a second record with same filename, must return ErrKeyConflict, and not overwrite.
	ef.BucketName = "bucket-2"
	err = testDB.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		if err := addExportFile(ctx, tx, ef); err != nil {
			if err == database.ErrKeyConflict {
				return nil // Expected result.
			}
			return err
		}
		return errors.New("missing expected ErrKeyConflict")
	})
	if err != nil {
		t.Fatal(err)
	}

	// Row must not be updated.
	got, err = exportDB.LookupExportFile(ctx, ef.Filename)
	if err != nil {
		t.Fatal(err)
	}
	if got.BucketName != wantBucketName {
		t.Fatalf("bucket name mismatch got %q, want %q", got.BucketName, wantBucketName)
	}
}

// TODO(jan25) add TestDeleteFilesBefore. Related to issue #241
