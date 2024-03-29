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

package export

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/util"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
)

func TestRandomInt(t *testing.T) {
	t.Parallel()

	expected := make(map[int]bool)
	for i := verifyapi.MinTransmissionRisk; i <= verifyapi.MaxTransmissionRisk; i++ {
		expected[i] = true
	}

	// Run through 1,000 iterations. To ensure the entire range can be hit.
	for i := 0; i < 1000; i++ {
		v, err := randomInt(verifyapi.MinTransmissionRisk, verifyapi.MaxTransmissionRisk)
		if err != nil {
			t.Fatalf("error getting random data")
		}
		if v < verifyapi.MinTransmissionRisk || v > verifyapi.MaxTransmissionRisk {
			t.Fatalf("random data outside expected bounds. %v <= %v <= %v",
				verifyapi.MinTransmissionRisk, v, verifyapi.MaxTransmissionRisk)
		}
		delete(expected, v)
	}
	if len(expected) != 0 {
		t.Fatalf("not all values hit in random range: %v", expected)
	}
}

func TestDoNotPadZeroLength(t *testing.T) {
	t.Parallel()

	exposures := make([]*publishmodel.Exposure, 0)
	exposures, generated, err := ensureMinNumExposures(exposures, "US", 1000, 100, 2000, time.Now())
	if err != nil {
		t.Fatalf("unepected error: %v", err)
	}
	if len(exposures) != 0 {
		t.Errorf("empty exposure list got padded, shouldn't have.")
	}
	if len(generated) != 0 {
		t.Errorf("generated data returned, should be empty")
	}
}

func TestEnsureMinExposures(t *testing.T) {
	t.Parallel()

	// Insert a few exposures - that will be used to base the interval information off of.
	exposures := []*publishmodel.Exposure{
		{
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(0),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
		{
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456 + 144,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(1),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
		{
			TransmissionRisk:      verifyapi.TransmissionRiskConfirmedStandard,
			IntervalNumber:        123456 - 144,
			IntervalCount:         144,
			DaysSinceSymptomOnset: proto.Int32(-1),
			ReportType:            verifyapi.ReportTypeConfirmed,
		},
	}

	for _, k := range exposures {
		k.ExposureKey = randomTEK(t)
	}

	numKeys := 100
	variance := 10
	m := make(map[int32]int)
	m[123456-144] = 1
	m[123456] = 1
	m[123456+144] = 1

	// pad the download.
	inputSize := len(exposures)
	exposures, generated, err := ensureMinNumExposures(exposures, "US", numKeys, variance, numKeys*10, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exposures) < numKeys || len(exposures) > (numKeys+variance+inputSize-1) {
		t.Errorf("wrong number of exposures, want: >=%v and <=%v, got: %v", numKeys, numKeys+variance, len(exposures))
	}
	if l, exp := len(generated), numKeys-inputSize; l < exp {
		t.Errorf("want keys >= %d, got: %d", exp, l)
	}

	for _, e := range exposures {
		m[e.IntervalNumber] = m[e.IntervalNumber] + 1
	}
	for k, v := range m {
		if v < 20 {
			t.Errorf("distribution not random, expected >= 20 keys with start interval %v, got %v", k, v)
		}
	}
}

func TestBatchExposures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		count int
	}{
		{
			"base_case",
			4,
		},
		{
			"a_lot",
			1000,
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := project.TestContext(t)
			testDB, _ := testDatabaseInstance.NewDatabase(t)
			testPublishDB := publishdb.New(testDB)

			config := Config{
				MinRecords:         1,
				PaddingRange:       0,
				MaxRecords:         1,
				TruncateWindow:     time.Hour,
				MaxInsertBatchSize: 100,
			}
			server := Server{
				config: &config,
				env:    serverenv.New(ctx, serverenv.WithDatabase(testDB)),
			}

			// Using a 1 hour truncate window
			// * one export batch lineage w/ 1 hour window
			// * one export batch with 4 hour window
			baseTime := time.Date(2020, 10, 28, 1, 0, 0, 0, time.UTC).Truncate(time.Hour)
			exposures := make([]*publishmodel.Exposure, 3*tc.count)
			for i := 0; i < tc.count; i++ {
				// home country non-traveler
				exposures[i*3] = &publishmodel.Exposure{
					ExposureKey:     randomTEK(t),
					Regions:         []string{"US"},
					IntervalNumber:  100,
					IntervalCount:   144,
					CreatedAt:       baseTime.Add(time.Duration(i%4) * time.Hour),
					LocalProvenance: true,
					Traveler:        false,
					ReportType:      verifyapi.ReportTypeClinical,
				}
				// foreign country traveler
				exposures[i*3+1] = &publishmodel.Exposure{
					ExposureKey:     randomTEK(t),
					Regions:         []string{"CA"},
					IntervalNumber:  100,
					IntervalCount:   144,
					CreatedAt:       baseTime.Add(time.Duration(i%4) * time.Hour),
					LocalProvenance: false,
					Traveler:        true,
					ReportType:      verifyapi.ReportTypeConfirmed,
				}
				// foreign country non-traveler
				exposures[i*3+2] = &publishmodel.Exposure{
					ExposureKey:     randomTEK(t),
					Regions:         []string{"CA"},
					IntervalNumber:  100,
					IntervalCount:   144,
					CreatedAt:       baseTime.Add(time.Duration(i%4) * time.Hour),
					LocalProvenance: false,
					Traveler:        false,
					ReportType:      verifyapi.ReportTypeConfirmed,
				}
			}
			if _, err := testPublishDB.InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
				Incoming:     exposures,
				RequireToken: false,
			}); err != nil {
				t.Fatalf("inserting exposures: %v", err)
			}
			// Make sure there are some revisions.
			revisions := make([]*publishmodel.Exposure, 0, 3*tc.count)
			for i, exp := range exposures {
				if exp.ReportType == verifyapi.ReportTypeClinical {
					revExp := &publishmodel.Exposure{
						ExposureKey:     exp.ExposureKey,
						Regions:         exp.Regions,
						IntervalNumber:  exp.IntervalNumber,
						IntervalCount:   exp.IntervalCount,
						CreatedAt:       baseTime.Add(time.Duration(i) * time.Hour).Add(time.Minute),
						LocalProvenance: exp.LocalProvenance,
						Traveler:        exp.Traveler,
						ReportType:      verifyapi.ReportTypeConfirmed,
					}
					revisions = append(revisions, revExp)
				}
			}
			if len(revisions) > 0 {
				if _, err := testPublishDB.InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
					Incoming:     revisions,
					RequireToken: false,
				}); err != nil {
					t.Fatalf("revising exposures: %v", err)
				}
			}

			homePlusTraveler := make(map[string]struct{})
			// Create the home country + traveler exports. 4, 1 per hour.
			for i := 0; i < 4; i++ {
				// Build the iteration criteria for the incremental batches.
				criteria := publishdb.IterateExposuresCriteria{
					SinceTimestamp:      baseTime.Add(time.Duration(i) * time.Hour),
					UntilTimestamp:      baseTime.Add(time.Duration(i) * time.Hour).Add(time.Hour),
					IncludeRegions:      []string{"US"},
					IncludeTravelers:    true,
					OnlyLocalProvenance: true,
				}

				groups, err := server.batchExposures(ctx, criteria, config.MaxRecords, "US")
				if err != nil {
					t.Fatalf("failed to read exposures: %v", err)
				}
				if len(groups) == 0 {
					t.Fatalf("export batch for %d should have found some keys", i)
				}

				for _, group := range groups {
					for _, exp := range group.exposures {
						b64 := exp.ExposureKeyBase64()
						if _, ok := homePlusTraveler[b64]; ok {
							t.Fatalf("hourly batch included duplicate key")
						}
						homePlusTraveler[b64] = struct{}{}
					}
				}
			}

			foreignNonTraveler := make(map[string]struct{})
			// Create the foreign, non traveler export.
			for i := 0; i < 4; i++ {
				// Build the iteration criteria for the incremental batches.
				criteria := publishdb.IterateExposuresCriteria{
					SinceTimestamp:      baseTime.Add(time.Duration(i) * time.Hour),
					UntilTimestamp:      baseTime.Add(time.Duration(i) * time.Hour).Add(time.Hour),
					IncludeRegions:      []string{"CA"}, // current list of foreign countries
					OnlyNonTravelers:    true,
					OnlyLocalProvenance: false,
				}

				groups, err := server.batchExposures(ctx, criteria, config.MaxRecords, "REMOTE")
				if err != nil {
					t.Fatalf("failed to read exposures: %v", err)
				}
				if len(groups) == 0 {
					t.Fatalf("export batch should have found some keys")
				}

				for _, group := range groups {
					for _, exp := range group.exposures {
						b64 := exp.ExposureKeyBase64()
						if _, ok := foreignNonTraveler[b64]; ok {
							t.Fatalf("hourly batch included duplicate key")
						}
						foreignNonTraveler[b64] = struct{}{}
					}
				}
			}

			// Run the 4 hour export for home+traveler
			{
				criteria := publishdb.IterateExposuresCriteria{
					SinceTimestamp:      baseTime,
					UntilTimestamp:      baseTime.Add(4 * time.Hour),
					IncludeRegions:      []string{"US"},
					IncludeTravelers:    true,
					OnlyLocalProvenance: true,
				}
				groups, err := server.batchExposures(ctx, criteria, config.MaxRecords, "US")
				if err != nil {
					t.Fatalf("failed to read exposures: %v", err)
				}
				if len(groups) == 0 {
					t.Fatalf("export batch should have keys")
				}

				homePlusTravelerRollup := make(map[string]struct{})
				for _, group := range groups {
					for _, exp := range group.exposures {
						b64 := exp.ExposureKeyBase64()
						if _, ok := homePlusTravelerRollup[b64]; ok {
							t.Fatalf("home rollup included duplicate key")
						}
						homePlusTravelerRollup[b64] = struct{}{}
					}
				}

				if diff := cmp.Diff(homePlusTraveler, homePlusTravelerRollup); diff != "" {
					t.Errorf("ReadExposures mismatch (-want, +got):\n%s", diff)
				}
			}

			// Run the 4 hour foreign, non-traveler
			{
				criteria := publishdb.IterateExposuresCriteria{
					SinceTimestamp:      baseTime,
					UntilTimestamp:      baseTime.Add(4 * time.Hour),
					IncludeRegions:      []string{"CA"}, // current list of foreign countries
					OnlyNonTravelers:    true,
					OnlyLocalProvenance: false,
				}
				groups, err := server.batchExposures(ctx, criteria, config.MaxRecords, "REMOTE")
				if err != nil {
					t.Fatalf("failed to read exposures: %v", err)
				}
				if len(groups) == 0 {
					t.Fatalf("export batch should have keys")
				}

				foreignNonTravelerRollup := make(map[string]struct{})
				for _, group := range groups {
					for _, exp := range group.exposures {
						b64 := exp.ExposureKeyBase64()
						if _, ok := foreignNonTravelerRollup[b64]; ok {
							t.Fatalf("home rollup included duplicate key")
						}
						foreignNonTravelerRollup[b64] = struct{}{}
					}
				}

				if diff := cmp.Diff(foreignNonTraveler, foreignNonTravelerRollup); diff != "" {
					t.Errorf("ReadExposures mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestVariableBatchMaxSize(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	testPublishDB := publishdb.New(testDB)

	// Using a 1 hour truncate window
	baseTime := time.Date(2020, 10, 28, 1, 0, 0, 0, time.UTC).Truncate(time.Hour)
	exposures := make([]*publishmodel.Exposure, 20)
	want := make(map[string]struct{})
	for i := 0; i < 20; i++ {
		// All keys aligned to the same hour.
		exposures[i] = &publishmodel.Exposure{
			ExposureKey:     randomTEK(t),
			Regions:         []string{"US"},
			IntervalNumber:  100,
			IntervalCount:   144,
			CreatedAt:       baseTime,
			LocalProvenance: true,
			Traveler:        false,
			ReportType:      verifyapi.ReportTypeClinical,
		}
		want[exposures[i].ExposureKeyBase64()] = struct{}{}
	}
	if _, err := testPublishDB.InsertAndReviseExposures(ctx, &publishdb.InsertAndReviseExposuresRequest{
		Incoming:     exposures,
		RequireToken: false,
	}); err != nil {
		t.Fatalf("inserting exposures: %v", err)
	}

	for batchSize := 1; batchSize <= len(exposures); batchSize++ {
		config := Config{
			MinRecords:         1,
			PaddingRange:       0,
			MaxRecords:         batchSize,
			TruncateWindow:     time.Hour,
			MaxInsertBatchSize: 100,
		}
		server := Server{
			config: &config,
			env:    serverenv.New(ctx, serverenv.WithDatabase(testDB)),
		}

		// Build the iteration criteria for the incremental batches.
		criteria := publishdb.IterateExposuresCriteria{
			SinceTimestamp:      baseTime,
			UntilTimestamp:      baseTime.Add(time.Hour),
			IncludeRegions:      []string{"US"}, // current list of foreign countries
			IncludeTravelers:    true,
			OnlyNonTravelers:    false,
			OnlyLocalProvenance: true,
		}

		groups, err := server.batchExposures(ctx, criteria, batchSize, "REMOTE")
		if err != nil {
			t.Fatalf("failed to read exposures: %v", err)
		}
		if len(groups) == 0 {
			t.Fatalf("export batch should have found some keys")
		}

		got := make(map[string]struct{})
		for _, group := range groups {
			for _, exp := range group.exposures {
				b64 := exp.ExposureKeyBase64()
				if _, ok := got[b64]; ok {
					t.Fatalf("hourly batch included duplicate key")
				}
				got[b64] = struct{}{}
			}
		}

		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("ReadExposures mismatch (-want, +got):\n%s", diff)
		}
	}
}

// randomTEK is like util.RandomTEK, but handles the error from tb.
func randomTEK(tb testing.TB) []byte {
	tb.Helper()

	b, err := util.RandomTEK()
	if err != nil {
		tb.Fatalf("failed to generate tek: %v", err)
	}
	return b
}

func TestExportFilename(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		m          *model.ExportBatch
		num        int32
		regenCount int64
		exp        string
	}{
		{
			name: "no_regn",
			m: &model.ExportBatch{
				FilenameRoot:   "v1",
				StartTimestamp: time.Unix(0, 0),
				EndTimestamp:   time.Unix(0, 0),
			},
			num:        1,
			regenCount: 0,
			exp:        "v1/0-0-00001.zip",
		},
		{
			name: "regen_2",
			m: &model.ExportBatch{
				FilenameRoot:   "v1",
				StartTimestamp: time.Unix(0, 0),
				EndTimestamp:   time.Unix(0, 0),
			},
			num:        2,
			regenCount: 2,
			exp:        "v1/2-2-00002.zip",
		},
		{
			name: "regen_3",
			m: &model.ExportBatch{
				FilenameRoot:   "v2",
				StartTimestamp: time.Unix(100, 0),
				EndTimestamp:   time.Unix(300, 0),
			},
			num:        1,
			regenCount: 3,
			exp:        "v2/103-303-00001.zip",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got, want := exportFilename(tc.m, tc.num, tc.regenCount), tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
