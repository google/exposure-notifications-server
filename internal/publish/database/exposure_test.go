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
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	pgx "github.com/jackc/pgx/v4"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var (
	approxTime               = cmp.Options{cmpopts.EquateApproxTime(time.Second)}
	ignoreUnexportedExposure = cmpopts.IgnoreUnexported(model.Exposure{})
)

func TestReadExposures(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	testPublishDB := New(testDB)
	ctx := context.Background()

	// Insert some Exposures.
	createdAt := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).Truncate(time.Microsecond)
	exposures := []*model.Exposure{
		{
			ExposureKey:     []byte("ABC123"),
			Regions:         []string{"US"},
			IntervalNumber:  100,
			IntervalCount:   144,
			CreatedAt:       createdAt,
			LocalProvenance: true,
		},
		{
			ExposureKey:     []byte("DEF456"),
			Regions:         []string{"US"},
			IntervalNumber:  244,
			IntervalCount:   144,
			CreatedAt:       createdAt,
			LocalProvenance: true,
		},
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, exposures); err != nil {
		t.Fatal(err)
	}

	keys := make([]string, 0, len(exposures))
	for _, e := range exposures {
		keys = append(keys, e.ExposureKeyBase64())
	}

	var readBack map[string]*model.Exposure
	err := testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		var err error
		readBack, err = testPublishDB.ReadExposures(ctx, tx, keys)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]*model.Exposure, 0, len(exposures))
	for _, v := range readBack {
		got = append(got, v)
	}

	sorter := cmp.Transformer("Sort", func(in []*model.Exposure) []*model.Exposure {
		out := append([]*model.Exposure(nil), in...) // Copy input to avoid mutating it
		sort.Slice(out, func(i int, j int) bool {
			return strings.Compare(out[i].ExposureKeyBase64(), out[j].ExposureKeyBase64()) <= 0
		})
		return out
	})

	if diff := cmp.Diff(exposures, got, approxTime, sorter, ignoreUnexportedExposure); diff != "" {
		t.Errorf("ReadExposures mismatch (-want, +got):\n%s", diff)
	}
}

func TestExposures(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	testPublishDB := New(testDB)
	ctx := context.Background()

	// Insert some Exposures.
	batchTime := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC).Truncate(time.Microsecond)
	exposures := []*model.Exposure{
		{
			ExposureKey:     []byte("ABC"),
			Regions:         []string{"US", "CA", "MX"},
			IntervalNumber:  18,
			IntervalCount:   0,
			CreatedAt:       batchTime,
			LocalProvenance: true,
		},
		{
			ExposureKey:     []byte("DEF"),
			Regions:         []string{"CA"},
			IntervalNumber:  118,
			IntervalCount:   1,
			CreatedAt:       batchTime.Add(1 * time.Hour),
			LocalProvenance: true,
		},
		{
			ExposureKey:     []byte("123"),
			IntervalNumber:  218,
			IntervalCount:   2,
			Regions:         []string{"MX", "CA"},
			CreatedAt:       batchTime.Add(2 * time.Hour),
			LocalProvenance: false,
		},
		{
			ExposureKey:     []byte("456"),
			IntervalNumber:  318,
			IntervalCount:   3,
			CreatedAt:       batchTime.Add(3 * time.Hour),
			Regions:         []string{"US"},
			LocalProvenance: false,
		},
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, exposures); err != nil {
		t.Fatal(err)
	}

	// Iterate over Exposures, with various criteria.
	for _, test := range []struct {
		criteria IterateExposuresCriteria
		want     []int
	}{
		{
			IterateExposuresCriteria{},
			[]int{0, 1, 2, 3},
		},
		{
			IterateExposuresCriteria{IncludeRegions: []string{"US"}},
			[]int{0, 3},
		},
		{
			IterateExposuresCriteria{ExcludeRegions: []string{"US"}},
			[]int{1, 2},
		},
		{
			IterateExposuresCriteria{IncludeRegions: []string{"CA"}, ExcludeRegions: []string{"MX"}},
			[]int{1},
		},
		{
			IterateExposuresCriteria{SinceTimestamp: exposures[2].CreatedAt},
			[]int{2, 3}, // SinceTimestamp is inclusive
		},
		{
			IterateExposuresCriteria{UntilTimestamp: exposures[2].CreatedAt},
			[]int{0, 1}, // UntilTimestamp is exclusive
		},
		{
			IterateExposuresCriteria{
				IncludeRegions: []string{"CA"},
				ExcludeRegions: []string{"MX"},
				SinceTimestamp: exposures[2].CreatedAt,
			},
			nil,
		},
	} {
		got, err := listExposures(ctx, testPublishDB, test.criteria)
		if err != nil {
			t.Fatalf("%+v: %v", test.criteria, err)
		}
		var want []*model.Exposure
		for _, i := range test.want {
			want = append(want, exposures[i])
		}
		if diff := cmp.Diff(want, got, ignoreUnexportedExposure); diff != "" {
			t.Errorf("%+v: mismatch (-want, +got):\n%s", test.criteria, diff)
		}
	}

	// Delete some exposures.
	gotN, err := testPublishDB.DeleteExposuresBefore(ctx, exposures[2].CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	wantN := int64(2) // The DeleteExposuresBefore time is exclusive, so we expect only the first two were deleted.
	if gotN != wantN {
		t.Errorf("DeleteExposuresBefore: deleted %d, want %d", gotN, wantN)
	}
	got, err := listExposures(ctx, testPublishDB, IterateExposuresCriteria{})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(exposures[2:], got, ignoreUnexportedExposure); diff != "" {
		t.Errorf("DeleteExposuresBefore: mismatch (-want, +got):\n%s", diff)
	}
}

func listExposures(ctx context.Context, db *PublishDB, c IterateExposuresCriteria) (_ []*model.Exposure, err error) {
	var exps []*model.Exposure
	if _, err := db.IterateExposures(ctx, c, func(e *model.Exposure) error {
		exps = append(exps, e)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to list exposures: %w", err)
	}
	return exps, nil
}

func TestReviseExposures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB := database.NewTestDatabase(t)
	pubDB := New(testDB)

	createdAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour)
	revisedAt := time.Now().UTC().Truncate(time.Hour)

	existingExp := &model.Exposure{
		ExposureKey:      []byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4},
		TransmissionRisk: verifyapi.TransmissionRiskClinical,
		AppPackageName:   "foo.bar",
		Regions:          []string{"US", "CA", "MX"},
		IntervalNumber:   100,
		IntervalCount:    144,
		CreatedAt:        createdAt,
		LocalProvenance:  true,
		ReportType:       verifyapi.ReportTypeClinical,
	}
	existingExp.SetDaysSinceSymptomOnset(0)
	if n, err := pubDB.InsertAndReviseExposures(ctx, []*model.Exposure{existingExp}); err != nil {
		t.Fatalf("error inserting exposures: %v", err)
	} else if n != 1 {
		t.Fatalf("wrong number of changed exposures, want: 1, got: %v", n)
	}

	// Modify the existing on in place.
	revisedExp := &model.Exposure{
		ExposureKey:      []byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 4},
		TransmissionRisk: verifyapi.TransmissionRiskConfirmedStandard,
		AppPackageName:   "foo.bar",
		Regions:          []string{"US", "CA", "MX"},
		IntervalNumber:   100,
		IntervalCount:    144,
		CreatedAt:        revisedAt,
		LocalProvenance:  true,
		ReportType:       verifyapi.ReportTypeConfirmed,
	}
	// Add a new TEK to insert.
	newExp := &model.Exposure{
		ExposureKey:      []byte{1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 5},
		TransmissionRisk: verifyapi.TransmissionRiskClinical,
		AppPackageName:   "foo.bar",
		Regions:          []string{"US", "CA", "MX"},
		IntervalNumber:   244,
		IntervalCount:    144,
		CreatedAt:        revisedAt,
		LocalProvenance:  true,
		ReportType:       verifyapi.ReportTypeConfirmed,
	}
	newExp.SetDaysSinceSymptomOnset(1)

	revisions := []*model.Exposure{revisedExp, newExp}
	if n, err := pubDB.InsertAndReviseExposures(ctx, revisions); err != nil {
		t.Fatalf("error revising exposures: %v", err)
	} else if n != 2 {
		t.Fatalf("wrong number of changed exposures, want: 2, got: %v", n)
	}

	// Read back and compare.
	expectedKeys := []string{existingExp.ExposureKeyBase64(), newExp.ExposureKeyBase64()}
	var got map[string]*model.Exposure
	err := testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		var err error
		got, err = pubDB.ReadExposures(ctx, tx, expectedKeys)
		return err
	})
	if err != nil {
		t.Fatalf("error reading exposures: %v", err)
	}

	if l := len(got); l != 2 {
		t.Fatalf("didn't read back both exposures, got: %v", l)
	}

	want := []*model.Exposure{existingExp, newExp}
	// need to modify the exitingExp to match what we expect to get back.
	existingExp.SetRevisedTransmissionRisk(verifyapi.TransmissionRiskConfirmedStandard)
	existingExp.SetRevisedAt(revisedAt)
	existingExp.SetRevisedReportType(verifyapi.ReportTypeConfirmed)

	for i := 0; i <= 1; i++ {
		if diff := cmp.Diff(want[i], got[want[i].ExposureKeyBase64()], ignoreUnexportedExposure); diff != "" {
			t.Errorf("mismatch (-want, +got):\n%s", diff)
		}
	}

	// Attempt to revise the already revised key.
	if _, err := pubDB.InsertAndReviseExposures(ctx, []*model.Exposure{revisedExp}); err == nil {
		t.Fatalf("expected error on revising already revised key, got nil")
	} else if !strings.Contains(err.Error(), "key has already been revised ") {
		t.Fatalf("wrong error, want 'invalid key revision request', got: %v", err)
	}
}

func TestIterateExposuresCursor(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	testPublishDB := New(testDB)
	ctx, cancel := context.WithCancel(context.Background())

	// Insert some Exposures.
	exposures := []*model.Exposure{
		{
			ExposureKey:    []byte("ABC"),
			Regions:        []string{"US", "CA", "MX"},
			IntervalNumber: 18,
		},
		{
			ExposureKey:    []byte("DEF"),
			Regions:        []string{"CA"},
			IntervalNumber: 118,
		},
		{
			ExposureKey:    []byte("123"),
			IntervalNumber: 218,
			Regions:        []string{"MX", "CA"},
		},
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, exposures); err != nil {
		t.Fatal(err)
	}
	// Iterate over them, canceling the context in the middle.
	var seen []*model.Exposure
	cursor, err := testPublishDB.IterateExposures(ctx, IterateExposuresCriteria{}, func(e *model.Exposure) error {
		seen = append(seen, e)
		if len(seen) == 2 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, wanted context.Canceled", err)
	}
	if diff := cmp.Diff(exposures[:2], seen, ignoreUnexportedExposure); diff != "" {
		t.Fatalf("exposures mismatch (-want, +got):\n%s", diff)
	}
	if want := encodeCursor("2"); cursor != want {
		t.Fatalf("cursor: got %q, want %q", cursor, want)
	}
	// Resume from the cursor.
	ctx = context.Background()
	seen = nil
	cursor, err = testPublishDB.IterateExposures(ctx, IterateExposuresCriteria{LastCursor: cursor},
		func(e *model.Exposure) error { seen = append(seen, e); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(exposures[2:], seen, ignoreUnexportedExposure); diff != "" {
		t.Fatalf("exposures mismatch (-want, +got):\n%s", diff)
	}
	if cursor != "" {
		t.Fatalf("cursor: got %q, want empty", cursor)
	}
}
