// Copyright 2020 the Exposure Notifications Server authors
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
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/pkg/database"

	"github.com/google/go-cmp/cmp"
	pgx "github.com/jackc/pgx/v4"
)

func TestAddRetrieveUpdateSignatureInfo(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

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
	if err := exDB.UpdateSignatureInfo(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err = exDB.GetSignatureInfo(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestLookupSignatureInfos(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	testTime := time.Now().UTC()
	create := []*model.SignatureInfo{
		{
			SigningKey:        "/kms/project/key/version/1",
			SigningKeyVersion: "1",
			SigningKeyID:      "310",
			EndTimestamp:      testTime.Add(-1 * time.Hour),
		},
		{
			SigningKey:        "/kms/project/key/version/2",
			SigningKeyVersion: "2",
			SigningKeyID:      "310",
			EndTimestamp:      testTime.Add(24 * time.Hour),
		},
		{
			SigningKey:        "/kms/project/key/version/3",
			SigningKeyVersion: "3",
			SigningKeyID:      "310",
		},
	}
	for _, si := range create {
		if err := New(testDB).AddSignatureInfo(ctx, si); err != nil {
			t.Fatalf("failed to add signature info %v: %v", si, err)
		}
	}

	ids := []int64{create[0].ID, create[1].ID, create[2].ID}
	got, err := New(testDB).LookupSignatureInfos(ctx, ids, testTime)
	if err != nil {
		t.Fatal(err)
	}

	// Specify the IDs we expect to be returned as effective based on
	// what was just created.
	want := []*model.SignatureInfo{
		create[2],
		create[1],
	}

	if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%v", diff)
	}
}

func TestAddGetUpdateExportConfig(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportDB := New(testDB)

	fromTime := time.Now()
	thruTime := fromTime.Add(6 * time.Hour)
	maxRecords := 5000
	want := &model.ExportConfig{
		BucketName:         "mocked",
		FilenameRoot:       "root",
		Period:             3 * time.Hour,
		OutputRegion:       "i1",
		InputRegions:       []string{"US"},
		IncludeTravelers:   true,
		ExcludeRegions:     []string{"EU"},
		From:               fromTime,
		Thru:               thruTime,
		SignatureInfoIDs:   []int64{42, 84},
		MaxRecordsOverride: &maxRecords,
	}
	if err := exportDB.AddExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := exportDB.GetExportConfig(ctx, want.ConfigID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Now update it.
	want.Period = 15 * time.Minute
	want.Thru = time.Time{}
	want.SignatureInfoIDs = []int64{1, 2, 3, 4, 5}
	want.InputRegions = []string{"US", "CA"}

	if err := exportDB.UpdateExportConfig(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err = exportDB.GetExportConfig(ctx, want.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestIterateExportConfigs(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	now := time.Now().Truncate(time.Microsecond)
	maxRecords := 27
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
			BucketName:         "b 3",
			FilenameRoot:       "done",
			From:               now.Add(-time.Hour),
			Thru:               now.Add(-time.Minute),
			MaxRecordsOverride: &maxRecords,
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
		ec.OutputRegion = "R"
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

	maxRecords := 42
	cases := []struct {
		name       string
		maxRecords *int
	}{
		{
			name:       "default_max_records",
			maxRecords: nil,
		},
		{
			name:       "max_records_override",
			maxRecords: &maxRecords,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)

			now := time.Now().Truncate(time.Microsecond)
			config := &model.ExportConfig{
				BucketName:         "mocked",
				FilenameRoot:       "root",
				Period:             time.Hour,
				OutputRegion:       "R",
				From:               now,
				Thru:               now.Add(time.Hour),
				SignatureInfoIDs:   []int64{1, 2, 3, 4},
				IncludeTravelers:   true,
				MaxRecordsOverride: tc.maxRecords,
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
					ConfigID:           config.ConfigID,
					BucketName:         config.BucketName,
					FilenameRoot:       config.FilenameRoot,
					OutputRegion:       config.OutputRegion,
					Status:             model.ExportBatchOpen,
					StartTimestamp:     start,
					EndTimestamp:       end,
					SignatureInfoIDs:   []int64{1, 2, 3, 4},
					IncludeTravelers:   true,
					MaxRecordsOverride: tc.maxRecords,
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
						got.OutputRegion != config.OutputRegion || got.BucketName != config.BucketName {
						t.Errorf("LeaseBatch: got (%d, %q, %q, %q), want (%d, %q, %q, %q)",
							got.ConfigID, got.BucketName, got.FilenameRoot, got.OutputRegion,
							config.ConfigID, config.BucketName, config.FilenameRoot, config.OutputRegion)
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
			err = testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
				return completeBatch(ctx, tx, batchID)
			})
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
		})
	}
}

func TestFinalizeBatch(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportDB := New(testDB)
	now := time.Now().Truncate(time.Microsecond)

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:   "some-bucket",
		FilenameRoot: "filename-root",
		Period:       time.Minute,
		OutputRegion: "US",
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
		OutputRegion:   ec.OutputRegion,
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
	ttl, _ := time.ParseDuration("20h")
	gotFiles, err := exportDB.LookupExportFiles(ctx, eb.ConfigID, ttl)
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
			BucketName:   eb.BucketName,
			Filename:     filename,
			BatchID:      eb.BatchID,
			OutputRegion: eb.OutputRegion,
			BatchNum:     i + 1,
			BatchSize:    batchSize,
			Status:       model.ExportBatchComplete,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("mismatch for %q (-want, +got):\n%s", filename, diff)
		}
	}

	// Check marking files for deletion.
	sleepTime := time.Second
	time.Sleep(sleepTime)
	if num, err := New(testDB).MarkExpiredFiles(ctx, eb.ConfigID, sleepTime); err != nil {
		t.Errorf("error marking files for deletion: %v", err)
	} else if num != len(gotFiles) {
		t.Errorf("expected to mark %d files expired, got %d", len(gotFiles), num)
	}
}

// TestTravelerKeys ensures traveler keys are pulled in when necessary.
func TestTravelerKeys(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	now := time.Now()

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:       "bucket-name",
		FilenameRoot:     "filename-root",
		Period:           3600 * time.Second,
		OutputRegion:     "US",
		From:             now.Add(-24 * time.Hour),
		IncludeTravelers: true,
	}
	if err := New(testDB).AddExportConfig(ctx, ec); err != nil {
		t.Fatal(err)
	}

	// Create a batch for two hours ago to one hour ago.
	startTimestamp := now.Truncate(time.Hour).Add(-2 * time.Hour)
	endTimestamp := startTimestamp.Add(time.Hour)
	eb := &model.ExportBatch{
		ConfigID:         ec.ConfigID,
		BucketName:       ec.BucketName,
		FilenameRoot:     ec.FilenameRoot,
		StartTimestamp:   startTimestamp,
		EndTimestamp:     endTimestamp,
		OutputRegion:     ec.OutputRegion,
		Status:           model.ExportBatchOpen,
		IncludeTravelers: ec.IncludeTravelers,
	}
	if err := New(testDB).AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}

	// Create traveler key out of the main region.
	trav := &publishmodel.Exposure{
		ExposureKey: []byte("aaa"),
		Regions:     []string{ec.OutputRegion + "A"},
		CreatedAt:   startTimestamp,
		Traveler:    true,
	}

	// Create non traveler key out of the main region.
	notTrav := &publishmodel.Exposure{
		ExposureKey: []byte("bbb"),
		Regions:     []string{ec.OutputRegion + "B"},
		CreatedAt:   endTimestamp,
	}

	// Add the keys to the database.
	if _, err := publishdb.New(testDB).InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
		Incoming:     []*publishmodel.Exposure{trav, notTrav},
		RequireToken: true,
	}); err != nil {
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
		IncludeRegions:   []string{leased.OutputRegion},
		SinceTimestamp:   leased.StartTimestamp,
		UntilTimestamp:   leased.EndTimestamp,
		IncludeTravelers: true,
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

// TestExcludeRegions ensures excluded regions are excluded.
func TestExcludeRegions(t *testing.T) {
	t.Parallel()

	inclRegion, exclRegion := "US", "EU"
	inclRegions, exclRegions, bothRegions := []string{inclRegion}, []string{exclRegion}, []string{inclRegion, exclRegion}
	now := time.Now()
	startTimestamp := now.Truncate(time.Hour).Add(-2 * time.Hour)
	endTimestamp := startTimestamp.Add(time.Hour)

	// Create person in main region.
	mainUser := &publishmodel.Exposure{
		ExposureKey: []byte("aaa"),
		Regions:     []string{inclRegion},
		CreatedAt:   startTimestamp,
	}

	// Create traveler&nonTraveler key out of the main region.
	extTravUser := &publishmodel.Exposure{
		ExposureKey: []byte("bbb"),
		Regions:     []string{exclRegion},
		CreatedAt:   startTimestamp,
		Traveler:    true,
	}
	extUser := &publishmodel.Exposure{
		ExposureKey: []byte("ccc"),
		Regions:     []string{exclRegion},
		CreatedAt:   startTimestamp,
	}

	cases := []struct {
		name        string
		users       []*publishmodel.Exposure
		inclRegions []string
		exclRegions []string
		nonTraveler bool
		want        []string
	}{
		{"1main,1ext", []*publishmodel.Exposure{mainUser, extUser}, inclRegions, exclRegions, false, []string{"aaa"}},
		{"1main,2ext", []*publishmodel.Exposure{mainUser, extUser, extTravUser}, inclRegions, exclRegions, false, []string{"aaa"}},
		{"1main,2ext,notravel", []*publishmodel.Exposure{mainUser, extUser, extTravUser}, inclRegions, nil, true, []string{"aaa"}},
		{"1main,2ext,notravel,2regions", []*publishmodel.Exposure{mainUser, extUser, extTravUser}, bothRegions, nil, true, []string{"aaa", "ccc"}},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)

			// Add a config.
			ec := &model.ExportConfig{
				BucketName:     "bucket-name",
				FilenameRoot:   "filename-root",
				Period:         3600 * time.Second,
				OutputRegion:   inclRegion,
				From:           now.Add(-24 * time.Hour),
				ExcludeRegions: []string{exclRegion},
			}
			if err := New(testDB).AddExportConfig(ctx, ec); err != nil {
				t.Fatal(err)
			}

			// Create a batch for two hours ago to one hour ago.
			eb := &model.ExportBatch{
				ConfigID:       ec.ConfigID,
				BucketName:     ec.BucketName,
				FilenameRoot:   ec.FilenameRoot,
				StartTimestamp: startTimestamp,
				EndTimestamp:   endTimestamp,
				OutputRegion:   ec.OutputRegion,
				Status:         model.ExportBatchOpen,
				ExcludeRegions: ec.ExcludeRegions,
			}
			if err := New(testDB).AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
				t.Fatal(err)
			}

			// Add the keys to the database.
			if _, err := publishdb.New(testDB).InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
				Incoming:     []*publishmodel.Exposure{mainUser, extUser, extTravUser},
				RequireToken: true,
			}); err != nil {
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
				IncludeRegions:   tc.inclRegions,
				SinceTimestamp:   leased.StartTimestamp,
				UntilTimestamp:   leased.EndTimestamp,
				ExcludeRegions:   tc.exclRegions,
				OnlyNonTravelers: tc.nonTraveler,
			}

			var got []*publishmodel.Exposure
			_, err = publishdb.New(testDB).IterateExposures(ctx, criteria, func(exp *publishmodel.Exposure) error {
				got = append(got, exp)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}

			keys := []string{}
			for _, exp := range got {
				keys = append(keys, string(exp.ExposureKey))
			}
			sort.Strings(keys)
			if !reflect.DeepEqual(tc.want, keys) {
				t.Fatalf("%v got = %v, want %v", tc.name, keys, tc.want)
			}
		})
	}
}

// TestKeysInBatch ensures that keys are fetched in the correct batch when they fall on boundary conditions.
func TestKeysInBatch(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	now := time.Now()

	// Add a config.
	ec := &model.ExportConfig{
		BucketName:   "bucket-name",
		FilenameRoot: "filename-root",
		Period:       3600 * time.Second,
		OutputRegion: "US",
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
		OutputRegion:   ec.OutputRegion,
		Status:         model.ExportBatchOpen,
	}
	if err := New(testDB).AddExportBatches(ctx, []*model.ExportBatch{eb}); err != nil {
		t.Fatal(err)
	}

	// Create key aligned with the StartTimestamp
	sek := &publishmodel.Exposure{
		ExposureKey: []byte("aaa"),
		Regions:     []string{ec.OutputRegion},
		CreatedAt:   startTimestamp,
	}

	// Create key aligned with the EndTimestamp
	eek := &publishmodel.Exposure{
		ExposureKey: []byte("bbb"),
		Regions:     []string{ec.OutputRegion},
		CreatedAt:   endTimestamp,
	}

	// Add the keys to the database.
	if _, err := publishdb.New(testDB).InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
		Incoming:     []*publishmodel.Exposure{sek, eek},
		RequireToken: true,
	}); err != nil {
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
		IncludeRegions: []string{leased.OutputRegion},
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

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	exportDB := New(testDB)

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
	err = testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
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
	err = testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		if err := addExportFile(ctx, tx, ef); err != nil {
			if errors.Is(err, database.ErrKeyConflict) {
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
