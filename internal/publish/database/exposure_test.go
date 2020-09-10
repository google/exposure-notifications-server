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
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/pb"
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
			Traveler:        true,
		},
		{
			ExposureKey:     []byte("DEF456"),
			Regions:         []string{"US"},
			IntervalNumber:  244,
			IntervalCount:   144,
			CreatedAt:       createdAt,
			LocalProvenance: true,
			Traveler:        true,
		},
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
		Incoming:     exposures,
		RequireToken: true,
	}); err != nil {
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

	// Test ReadExposures of an empty list.
	{
		var keys []string
		var readBack map[string]*model.Exposure
		err := testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			var err error
			readBack, err = testPublishDB.ReadExposures(ctx, tx, keys)
			return err
		})
		if err != nil {
			t.Fatalf("unable to read empty list of keys: %v", err)
		}
		if l := len(readBack); l != 0 {
			t.Fatalf("want readBack len=0, got: %v", l)
		}
	}

	// Test ReadExposures of a key that doesn't exist in DB.
	{
		keys := []string{base64.StdEncoding.EncodeToString([]byte{19, 8, 7, 6, 5, 4, 3, 2, 1})}
		var readBack map[string]*model.Exposure
		err := testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			var err error
			readBack, err = testPublishDB.ReadExposures(ctx, tx, keys)
			return err
		})
		if err != nil {
			t.Fatalf("unable to read keys that don't exist: %v", err)
		}
		if l := len(readBack); l != 0 {
			t.Fatalf("want readBack len=0, got: %v", l)
		}
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
			Traveler:        true,
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
			Traveler:        true,
			LocalProvenance: false,
		},
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
		Incoming:     exposures,
		RequireToken: true,
	}); err != nil {
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
		{
			IterateExposuresCriteria{OnlyTravelers: true},
			[]int{1, 3},
		},
		{
			IterateExposuresCriteria{IncludeRegions: []string{"CA"}, IncludeTravelers: true},
			[]int{0, 1, 2, 3},
		},
		{
			IterateExposuresCriteria{OnlyLocalProvenance: true, OnlyTravelers: true},
			[]int{1},
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

func testRandomTEK(tb testing.TB) []byte {
	tb.Helper()

	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil {
		tb.Fatal(err)
	}
	if n < 16 {
		tb.Fatalf("expected %v to be %v", n, 16)
	}
	return b
}

func testExposure(tb testing.TB) *model.Exposure {
	tb.Helper()

	exposure := &model.Exposure{
		ExposureKey:      testRandomTEK(tb),
		TransmissionRisk: verifyapi.TransmissionRiskClinical,
		AppPackageName:   "foo.bar",
		Regions:          []string{"US", "CA", "MX"},
		IntervalNumber:   100,
		IntervalCount:    144,
		CreatedAt:        time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Hour),
		LocalProvenance:  true,
		ReportType:       verifyapi.ReportTypeClinical,
	}
	exposure.SetDaysSinceSymptomOnset(0)
	return exposure
}

func TestReviseExposures(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB := database.NewTestDatabase(t)
	pubDB := New(testDB)

	// Attempt to revise without a revision token - this should fail.
	t.Run("revise_without_token", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Change and revise
		exposure.TransmissionRisk = verifyapi.TransmissionRiskConfirmedStandard
		exposure.ReportType = verifyapi.ReportTypeConfirmed

		if _, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			RequireToken: true,
		}); !errors.Is(err, ErrNoRevisionToken) {
			t.Errorf("expected %#v to be %#v", err, ErrNoRevisionToken)
		}
	})

	// Attempt to revise with a revision token that doesn't include the TEK - this
	// should fail.
	t.Run("revise_with_token_without_matching_tek", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Change and revise
		exposure.TransmissionRisk = verifyapi.TransmissionRiskConfirmedStandard
		exposure.ReportType = verifyapi.ReportTypeConfirmed

		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: testRandomTEK(t), // TEK mismatch
					IntervalNumber:       exposure.IntervalNumber,
					IntervalCount:        exposure.IntervalCount,
				},
			},
		}

		if _, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			Token:        token,
			RequireToken: true,
		}); !errors.Is(err, ErrExistingKeyNotInToken) {
			t.Fatalf("expected %#v to be %#v", err, ErrExistingKeyNotInToken)
		}
	})

	// Attempt to revise with a revision token that has the wrong metadata - this
	// should fail.
	t.Run("revise_with_token_wrong_metadata", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: exposure.ExposureKey,
					IntervalNumber:       exposure.IntervalNumber + 1, // Changed, should cause failure
					IntervalCount:        exposure.IntervalCount,
				},
			},
		}

		if _, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			Token:        token,
			RequireToken: true,
		}); !errors.Is(err, ErrRevisionTokenMetadataMismatch) {
			t.Fatalf("expected %#v to be %#v", err, ErrRevisionTokenMetadataMismatch)
		}
	})

	// Attempt to modify metadata on incoming keys - this should fail.
	t.Run("revise_metadata_incoming_keys", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Change metadata - should make request invalid
		exposure.IntervalCount = 124

		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: exposure.ExposureKey,
					IntervalNumber:       exposure.IntervalNumber,
					IntervalCount:        exposure.IntervalCount,
				},
			},
		}

		if _, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			Token:        token,
			RequireToken: true,
		}); !errors.Is(err, ErrIncomingMetadataMismatch) {
			t.Fatalf("expected %#v to be %#v", err, ErrIncomingMetadataMismatch)
		}
	})

	// Attempt to revise where a subset of the TEKs match the revision token and
	// partial revisions are permitted. This should succeed.
	t.Run("revise_subset_of_keys_partial", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure1 := testExposure(t)
		insertResp1, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure1},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp1.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Change the diagnosis
		exposure1.ReportType = verifyapi.ReportTypeConfirmed

		// Create new exposure which is not included in the revision token, but
		// should also not return an error due to partial revisions being permitted.
		exposure2 := testExposure(t)
		insertResp2, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure2},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp2.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Create the revision token, only including the first token.
		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: exposure1.ExposureKey,
					IntervalNumber:       exposure1.IntervalNumber,
					IntervalCount:        exposure1.IntervalCount,
				},
			},
		}

		// Attempt to revise
		resp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:              []*model.Exposure{exposure1, exposure2},
			Token:                 token,
			RequireToken:          true,
			AllowPartialRevisions: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(resp.Inserted), 0; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := int(resp.Revised), 1; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
		if got, want := int(resp.Dropped), 1; got != want {
			t.Errorf("expected %d to be %d", got, want)
		}
	})

	t.Run("revise_already_revised", func(t *testing.T) {
		t.Parallel()

		// Insert the exposure
		exposure := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 1; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Revise the exposure
		exposure.ReportType = verifyapi.ReportTypeConfirmed

		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: exposure.ExposureKey,
					IntervalNumber:       exposure.IntervalNumber,
					IntervalCount:        exposure.IntervalCount,
				},
			},
		}

		reviseResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			Token:        token,
			RequireToken: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(reviseResp.Revised), 1; got != want {
			t.Errorf("expected %d to be %d: %#v", got, want, reviseResp)
		}

		// Attempt to revise the same key again - this should fail.
		if _, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:     []*model.Exposure{exposure},
			Token:        token,
			RequireToken: true,
		}); !errors.Is(err, model.ErrorKeyAlreadyRevised) {
			t.Errorf("expected %#v to be %#v", err, model.ErrorKeyAlreadyRevised)
		}
	})

	t.Run("revise_partial_new_flow", func(t *testing.T) {
		t.Parallel()

		// Insert 2 exposures
		exposure1 := testExposure(t)
		exposure2 := testExposure(t)
		insertResp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming: []*model.Exposure{exposure1, exposure2},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(insertResp.Inserted), 2; got != want {
			t.Fatalf("expected %d to be %d", got, want)
		}

		// Change the first exposure
		exposure1.ReportType = verifyapi.ReportTypeConfirmed

		// Change the second exposure
		exposure2.ReportType = verifyapi.ReportTypeConfirmed

		// Create another new exposure to insert
		exposure3 := testExposure(t)
		exposure3.SetDaysSinceSymptomOnset(1)
		exposure3.IntervalNumber = 244
		exposure3.IntervalCount = 144
		exposure3.ReportType = verifyapi.ReportTypeConfirmed

		// Create a revision token that includes exposure1, but not exposure2 or
		// exposure3.
		token := &pb.RevisionTokenData{
			RevisableKeys: []*pb.RevisableKey{
				{
					TemporaryExposureKey: exposure1.ExposureKey,
					IntervalNumber:       exposure1.IntervalNumber,
					IntervalCount:        exposure1.IntervalCount,
				},
			},
		}

		// Exposure2 should be dropped since it exists and is not in the database,
		// but exposure3 should be inserted because its new.
		resp, err := pubDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
			Incoming:              []*model.Exposure{exposure1, exposure2, exposure3},
			Token:                 token,
			RequireToken:          true,
			AllowPartialRevisions: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := int(resp.Inserted), 1; got != want {
			// exposure3
			t.Errorf("expected inserted %d to be %d", got, want)
		}
		if got, want := int(resp.Revised), 1; got != want {
			// exposure1
			t.Errorf("expected revised %d to be %d", got, want)
		}
		if got, want := int(resp.Dropped), 1; got != want {
			// exposure2
			t.Errorf("expected dropped %d to be %d", got, want)
		}
		if got, want := len(resp.Exposures), 2; got != want {
			t.Errorf("expected exposures %d to be %d", got, want)
		}

		// Ensure the returned exposures don't include exposure 2 which should have
		// been dropped. This will ensure it's not included in the revision token as
		// well.
		gotExposures := make([]string, len(resp.Exposures))
		for i, e := range resp.Exposures {
			gotExposures[i] = e.ExposureKeyBase64()
		}
		sort.Strings(gotExposures)
		wantExposures := []string{
			exposure1.ExposureKeyBase64(),
			exposure3.ExposureKeyBase64(),
		}
		sort.Strings(wantExposures)
		if got, want := gotExposures, wantExposures; !reflect.DeepEqual(got, want) {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		// Read back and ensure they actually went in the database.
		expectedTEKs := []string{
			exposure1.ExposureKeyBase64(),
			exposure2.ExposureKeyBase64(),
			exposure3.ExposureKeyBase64(),
		}
		var got map[string]*model.Exposure
		if err := testDB.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
			var err error
			got, err = pubDB.ReadExposures(ctx, tx, expectedTEKs)
			return err
		}); err != nil {
			t.Fatal(err)
		}

		// Ensure exposure1 was revised.
		gotExposure1, ok := got[exposure1.ExposureKeyBase64()]
		if !ok {
			t.Fatal("exposure1 is missing in response")
		}
		if got, want := gotExposure1.RevisedReportType, verifyapi.ReportTypeConfirmed; got == nil || *got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		// Ensure exposure2 was NOT revised (it was not in the revision token and
		// should have been dropped).
		gotExposure2, ok := got[exposure2.ExposureKeyBase64()]
		if !ok {
			t.Fatal("exposure2 is missing in response")
		}
		if got, want := gotExposure2.ReportType, verifyapi.ReportTypeClinical; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}

		// Ensure exposure3 was inserted.
		gotExposure3, ok := got[exposure3.ExposureKeyBase64()]
		if !ok {
			t.Fatal("exposure3 is missing in response")
		}
		if got, want := gotExposure3.IntervalNumber, exposure3.IntervalNumber; got != want {
			t.Errorf("expected %#v to be %#v", got, want)
		}
	})
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
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, &InsertAndReviseExposuresRequest{
		Incoming:     exposures,
		RequireToken: true,
	}); err != nil {
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
	if err == nil {
		t.Fatal("expected error")
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
