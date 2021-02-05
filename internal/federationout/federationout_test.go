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

package federationout

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/pb/federation"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	// listsAsSets are cmp.Options to remove list ordering from diffs of protos.
	listsAsSets = []cmp.Option{
		protocmp.Transform(),
		protocmp.SortRepeatedFields(&federation.FederationFetchResponse{}, "keys"),
		protocmp.SortRepeatedFields(&federation.FederationFetchResponse{}, "revisedKeys"),
	}

	aaa = &federation.ExposureKey{ExposureKey: []byte("aaaaaaaaaaaaaaaa"), IntervalNumber: 1}
	bbb = &federation.ExposureKey{ExposureKey: []byte("bbbbbbbbbbbbbbbb"), IntervalNumber: 2}
	ccc = &federation.ExposureKey{ExposureKey: []byte("cccccccccccccccc"), IntervalNumber: 3}
	ddd = &federation.ExposureKey{ExposureKey: []byte("dddddddddddddddd"), IntervalNumber: 4}
)

// makeExposure returns a mock model.Exposure.
func makeExposure(diagKey *federation.ExposureKey, reportType string, region string, traveler bool) *model.Exposure {
	return &model.Exposure{
		ExposureKey:      diagKey.ExposureKey,
		TransmissionRisk: model.ReportTypeTransmissionRisk(reportType, 0),
		AppPackageName:   "default",
		Regions:          []string{region},
		Traveler:         traveler,
		IntervalNumber:   diagKey.IntervalNumber,
		IntervalCount:    144,
		CreatedAt:        time.Unix(int64(diagKey.IntervalNumber*100), 0), // Make unique from IntervalNumber.
		LocalProvenance:  true,
		ReportType:       reportType,
	}
}

func addRegions(e *model.Exposure, regions ...string) *model.Exposure {
	e.AddMissingRegions(regions)
	return e
}

func addDaysSinceOnset(e *model.Exposure, daysSince int32) *model.Exposure {
	e.SetDaysSinceSymptomOnset(daysSince)
	return e
}

func reviseExposure(e *model.Exposure, newReportType string, clearOnset bool) *model.Exposure {
	e.SetRevisedReportType(newReportType)
	if e.HasDaysSinceSymptomOnset() && !clearOnset {
		e.SetRevisedDaysSinceSymptomOnset(*e.DaysSinceSymptomOnset)
	}
	e.SetRevisedTransmissionRisk(model.ReportTypeTransmissionRisk(newReportType, 0))
	e.SetRevisedAt(e.CreatedAt.Add(time.Second))
	return e
}

// timeout is used by testIterator to indicate that a timeout signal should be sent.
type timeout struct{}

type testIterator struct {
	t *testing.T

	primary []interface{}
	revised []interface{}

	stage uint
}

// A fetch request will make 2 separate calls to the iterator function. The first is for primary keys
// and the second is for revised keys.
func (t *testIterator) provideInput(_ context.Context, _ publishdb.IterateExposuresCriteria, f publishdb.IteratorFunction) (string, error) {
	t.t.Helper()

	var elements []interface{}
	if t.stage == 0 {
		elements = make([]interface{}, len(t.primary))
		copy(elements, t.primary)
		t.t.Logf("providing %v primary keys", len(elements))
	} else if t.stage == 1 {
		elements = make([]interface{}, len(t.revised))
		copy(elements, t.revised)
		t.t.Logf("providing %v revised keys", len(elements))
	}
	t.stage++

	var cursor string
	for _, el := range elements {
		switch v := el.(type) {
		case *model.Exposure:
			// Set the cursor to the most recent diagnosis key, suffixed with "_cursor".
			cursor = string(v.ExposureKey) + "_cursor"
			if err := f(v); err != nil {
				return cursor, err
			}
		case timeout:
			return cursor, context.Canceled
		default:
			panic("bad element")
		}
	}
	return cursor, nil
}

// TestFetch tests the fetch() function.
func TestFetch(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		includeRegions    []string
		excludeRegions    []string
		onlyTravelers     bool
		iterations        []interface{}
		revisedIterations []interface{}
		want              *federation.FederationFetchResponse
	}{
		{
			name:           "no_results",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			want: &federation.FederationFetchResponse{
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 1,
					},
					RevisedKeyCursor: &federation.Cursor{},
				},
			},
		},
		{
			name:           "basic_primary_results",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			iterations: []interface{}{
				addDaysSinceOnset(makeExposure(aaa, "confirmed", "US", false), 1),
				makeExposure(bbb, "confirmed", "US", true),
				makeExposure(ccc, "likely", "US", false),
				addDaysSinceOnset(makeExposure(ddd, "likely", "US", true), 1),
			},
			revisedIterations: []interface{}{},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 1,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              bbb.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           bbb.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 0,
						HasSymptomOnset:          false,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              ccc.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           ccc.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: 0,
						HasSymptomOnset:          false,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              ddd.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           ddd.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: 1,
						HasSymptomOnset:          true,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
				},
				PartialResponse: false,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 401,
						NextToken: "",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 1,
						NextToken: "",
					},
				},
			},
		},
		{
			name:           "primary_and_revised",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			iterations: []interface{}{
				addDaysSinceOnset(makeExposure(aaa, "confirmed", "US", false), 2),
			},
			revisedIterations: []interface{}{
				reviseExposure(addDaysSinceOnset(makeExposure(bbb, "likely", "US", true), 1), "confirmed", false),
			},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 2,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
				},
				RevisedKeys: []*federation.ExposureKey{
					{
						ExposureKey:              bbb.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           bbb.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 1,
						HasSymptomOnset:          true,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
				},
				PartialResponse: false,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 101,
						NextToken: "",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 202,
						NextToken: "",
					},
				},
			},
		},
		{
			name:           "revoke_key",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			iterations: []interface{}{
				addDaysSinceOnset(makeExposure(aaa, "likely", "US", false), 2),
			},
			revisedIterations: []interface{}{
				reviseExposure(addDaysSinceOnset(makeExposure(aaa, "likely", "US", false), 2), "negative", true),
			},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: 2,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
				},
				RevisedKeys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskNegative,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_REVOKED,
						DaysSinceOnsetOfSymptoms: 0,
						HasSymptomOnset:          false,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
				},
				PartialResponse: false,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 101,
						NextToken: "",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 102,
						NextToken: "",
					},
				},
			},
		},
		{
			name:           "partial_primary_result",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			iterations: []interface{}{
				addDaysSinceOnset(makeExposure(aaa, "likely", "US", true), -1),
				addDaysSinceOnset(makeExposure(bbb, "confirmed", "US", false), -2),
				timeout{},
				addDaysSinceOnset(makeExposure(ccc, "confirmed", "US", false), 2),
			},
			revisedIterations: []interface{}{
				// In this test, we shouldn't get here because of the timeout in the initial set.
				reviseExposure(addDaysSinceOnset(makeExposure(aaa, "likely", "US", true), 2), "confirmed", true),
			},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: -1,
						HasSymptomOnset:          true,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              bbb.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           bbb.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: -2,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
				},
				RevisedKeys:     []*federation.ExposureKey{},
				PartialResponse: true,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 200,
						NextToken: "bbbbbbbbbbbbbbbb_cursor",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 0,
						NextToken: "",
					},
				},
			},
		},
		{
			name:           "partial_revised_result",
			includeRegions: []string{"US"},
			excludeRegions: []string{},
			iterations: []interface{}{
				addDaysSinceOnset(makeExposure(aaa, "likely", "US", true), -1),
				addDaysSinceOnset(makeExposure(bbb, "likely", "US", false), -2),
			},
			revisedIterations: []interface{}{
				// In this test, we shouldn't get here because of the timeout in the initial set.
				reviseExposure(addDaysSinceOnset(makeExposure(aaa, "likely", "US", true), -1), "confirmed", false),
				timeout{},
				reviseExposure(addDaysSinceOnset(makeExposure(bbb, "likely", "US", true), -1), "confirmed", false),
			},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: -1,
						HasSymptomOnset:          true,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              bbb.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           bbb.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: -2,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US"},
					},
				},
				RevisedKeys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: -1,
						HasSymptomOnset:          true,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
				},
				PartialResponse: true,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 201,
						NextToken: "",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 101,
						NextToken: "aaaaaaaaaaaaaaaa_cursor",
					},
				},
			},
		},
		{
			name:           "multiple_regions",
			includeRegions: []string{"US", "CA"},
			excludeRegions: []string{"CH"},
			iterations: []interface{}{
				addDaysSinceOnset(addRegions(makeExposure(aaa, "confirmed", "US", false), "CA"), 1),
				addRegions(makeExposure(bbb, "confirmed", "US", true), "CH"),
				addRegions(makeExposure(ccc, "likely", "US", false), "CA", "CH", "MX"),
			},
			revisedIterations: []interface{}{},
			want: &federation.FederationFetchResponse{
				Keys: []*federation.ExposureKey{
					{
						ExposureKey:              aaa.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           aaa.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 1,
						HasSymptomOnset:          true,
						Traveler:                 false,
						Regions:                  []string{"US", "CA"},
					},
					{
						ExposureKey:              bbb.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskConfirmedStandard,
						IntervalNumber:           bbb.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_TEST,
						DaysSinceOnsetOfSymptoms: 0,
						HasSymptomOnset:          false,
						Traveler:                 true,
						Regions:                  []string{"US"},
					},
					{
						ExposureKey:              ccc.ExposureKey,
						TransmissionRisk:         verifyapi.TransmissionRiskClinical,
						IntervalNumber:           ccc.IntervalNumber,
						IntervalCount:            144,
						ReportType:               federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS,
						DaysSinceOnsetOfSymptoms: 0,
						HasSymptomOnset:          false,
						Traveler:                 false,
						Regions:                  []string{"US", "CA"},
					},
				},
				PartialResponse: false,
				NextFetchState: &federation.FetchState{
					KeyCursor: &federation.Cursor{
						Timestamp: 301,
						NextToken: "",
					},
					RevisedKeyCursor: &federation.Cursor{
						Timestamp: 1,
						NextToken: "",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := project.TestContext(t)
			env := serverenv.New(ctx)
			server := Server{
				env: env,
				config: &Config{
					MaxRecords: 100,
				},
			}
			req := &federation.FederationFetchRequest{
				IncludeRegions:      tc.includeRegions,
				ExcludeRegions:      tc.excludeRegions,
				OnlyLocalProvenance: true,
				State:               nil,
			}

			fakeDB := testIterator{
				t:       t,
				primary: tc.iterations,
				revised: tc.revisedIterations,
			}

			got, err := server.fetch(project.TestContext(t), req, fakeDB.provideInput, time.Now())
			if err != nil {
				t.Fatalf("fetch() returned err=%v, want err=nil", err)
			}
			if diff := cmp.Diff(tc.want, got, listsAsSets...); diff != "" {
				t.Errorf("fetch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestRawToken tests rawToken().
func TestRawToken(t *testing.T) {
	t.Parallel()

	want := "Abc123"
	md := metadata.New(map[string]string{"authorization": fmt.Sprintf("Bearer %s", want)})
	ctx := metadata.NewIncomingContext(project.TestContext(t), md)

	got, err := rawToken(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("rawToken()=%q, want=%q", got, want)
	}
}

// TestRawTokenErrors tests error responses from rawToken().
func TestRawTokenErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		mdMap map[string][]string
	}{
		{
			name:  "no metadata",
			mdMap: map[string][]string{},
		},
		{
			name:  "missing authorization header",
			mdMap: map[string][]string{"not-auth": {"foo"}},
		},
		{
			name:  "empty authorization header",
			mdMap: map[string][]string{"authorization": {}},
		},
		{
			name:  "too many authorization headers",
			mdMap: map[string][]string{"authorization": {"Bearer foo", "Bearer bar"}},
		},
		{
			name:  "incorrect authorization prefix",
			mdMap: map[string][]string{"authorization": {"not-Bearer foo"}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(project.TestContext(t), tc.mdMap)
			_, gotErr := rawToken(ctx)
			if gotErr == nil {
				t.Errorf("missing error response")
			}
		})
	}
}

// TestIntersect tests intersect().
func TestIntersect(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		aa   []string
		bb   []string
		want []string
	}{
		{
			name: "empty",
			aa:   []string{},
			bb:   []string{},
			want: nil,
		},
		{
			name: "aa only values",
			aa:   []string{"1", "2"},
			bb:   []string{},
			want: nil,
		},
		{
			name: "bb only values",
			aa:   []string{},
			bb:   []string{"1", "2"},
			want: nil,
		},
		{
			name: "mutually exclusive",
			aa:   []string{"1", "2"},
			bb:   []string{"7", "8", "9"},
			want: nil,
		},
		{
			name: "full overlap",
			aa:   []string{"1", "2"},
			bb:   []string{"1", "2"},
			want: []string{"1", "2"},
		},
		{
			name: "partial overlap",
			aa:   []string{"1", "2", "3"},
			bb:   []string{"2", "3", "4", "5"},
			want: []string{"2", "3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := intersect(tc.aa, tc.bb)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

// TestUnion tests union().
func TestUnion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		aa   []string
		bb   []string
		want []string
	}{
		{
			name: "empty",
			aa:   []string{},
			bb:   []string{},
			want: []string{},
		},
		{
			name: "aa only values",
			aa:   []string{"1", "2"},
			bb:   []string{},
			want: []string{"1", "2"},
		},
		{
			name: "bb only values",
			aa:   []string{},
			bb:   []string{"1", "2"},
			want: []string{"1", "2"},
		},
		{
			name: "mutually exclusive",
			aa:   []string{"1", "2"},
			bb:   []string{"7", "8", "9"},
			want: []string{"1", "2", "7", "8", "9"},
		},
		{
			name: "full overlap",
			aa:   []string{"1", "2"},
			bb:   []string{"1", "2"},
			want: []string{"1", "2"},
		},
		{
			name: "partial overlap",
			aa:   []string{"1", "2", "3"},
			bb:   []string{"2", "3", "4", "5"},
			want: []string{"1", "2", "3", "4", "5"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := union(tc.aa, tc.bb)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
