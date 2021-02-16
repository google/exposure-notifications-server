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

package federationin

import (
	"context"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/federationin/database"
	"github.com/google/exposure-notifications-server/internal/federationin/model"
	"github.com/google/exposure-notifications-server/internal/project"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"

	"github.com/google/exposure-notifications-server/internal/pb/federation"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
)

var (
	syncID  int64  = 999
	queryID string = "sharing"

	aaa = &federation.ExposureKey{ExposureKey: []byte("aaaaaaaaaaaaaaaa"), IntervalNumber: 1, IntervalCount: 144}
	bbb = &federation.ExposureKey{ExposureKey: []byte("bbbbbbbbbbbbbbbb"), IntervalNumber: 2, IntervalCount: 144}
	ccc = &federation.ExposureKey{ExposureKey: []byte("cccccccccccccccc"), IntervalNumber: 3, IntervalCount: 144}
	ddd = &federation.ExposureKey{ExposureKey: []byte("dddddddddddddddd"), IntervalNumber: 4, IntervalCount: 144}
)

// Makes a copy of the exposure key to avoid side effects between tests.
func copyExposureKey(e *federation.ExposureKey) *federation.ExposureKey {
	cpy := federation.ExposureKey{
		ExposureKey:              make([]byte, 16),
		TransmissionRisk:         e.TransmissionRisk,
		IntervalNumber:           e.IntervalNumber,
		IntervalCount:            e.IntervalCount,
		ReportType:               e.ReportType,
		DaysSinceOnsetOfSymptoms: e.DaysSinceOnsetOfSymptoms,
		HasSymptomOnset:          e.HasSymptomOnset,
		Traveler:                 e.Traveler,
		Regions:                  make([]string, len(e.Regions)),
	}
	copy(cpy.ExposureKey, e.ExposureKey)
	copy(cpy.Regions, e.Regions)
	return &cpy
}

func setRegions(e *federation.ExposureKey, regions ...string) *federation.ExposureKey {
	e.Regions = regions
	return e
}

func setIntervalNumber(e *federation.ExposureKey, interval int32) *federation.ExposureKey {
	e.IntervalNumber = interval
	return e
}

func setTransmissionRisk(e *federation.ExposureKey, tr int32) *federation.ExposureKey {
	e.TransmissionRisk = tr
	return e
}

func setReportType(e *federation.ExposureKey, reportType federation.ExposureKey_ReportType) *federation.ExposureKey {
	e.ReportType = reportType
	return e
}

// remoteFetchServer mocks responses from the remote federation server.
type remoteFetchServer struct {
	responses []*federation.FederationFetchResponse
	gotTokens []*federation.FetchState
	index     int
}

func (r *remoteFetchServer) fetch(ctx context.Context, req *federation.FederationFetchRequest, opts ...grpc.CallOption) (*federation.FederationFetchResponse, error) {
	r.gotTokens = append(r.gotTokens, req.State)
	if r.responses == nil || r.index > len(r.responses) {
		return &federation.FederationFetchResponse{}, nil
	}
	response := r.responses[r.index]
	r.index++
	return response, nil
}

// publishDB mocks the database, recording exposure insertions.
type publishDB struct {
	exposures []*publishmodel.Exposure
}

func (idb *publishDB) insertExposures(ctx context.Context, req *publishdb.InsertAndReviseExposuresRequest) (*publishdb.InsertAndReviseExposuresResponse, error) {
	idb.exposures = append(idb.exposures, req.Incoming...)
	return &publishdb.InsertAndReviseExposuresResponse{
		Exposures: req.Incoming,
		Inserted:  uint32(len(req.Incoming)),
		Revised:   uint32(len(req.Incoming)), // mock doesn't know if it is revising or inserting.
	}, nil
}

// syncDB mocks the database, recording start and complete invocations for a sync record.
type syncDB struct {
	syncStarted         bool
	syncCompleted       bool
	completed           time.Time
	maxTimestamp        time.Time
	maxRevisedTimestamp time.Time
	totalInserted       int
}

func (sdb *syncDB) startFederationSync(ctx context.Context, query *model.FederationInQuery, start time.Time) (int64, database.FinalizeSyncFn, error) {
	sdb.syncStarted = true
	timerStart := time.Now()
	return syncID, func(state *federation.FetchState, q *model.FederationInQuery, totalInserted int) error {
		sdb.syncCompleted = true
		sdb.completed = start.Add(time.Since(timerStart))
		if state.KeyCursor != nil {
			sdb.maxTimestamp = time.Unix(state.KeyCursor.Timestamp, 0).UTC()
		}
		if state.RevisedKeyCursor != nil {
			sdb.maxRevisedTimestamp = time.Unix(state.RevisedKeyCursor.Timestamp, 0).UTC()
		}
		sdb.totalInserted = totalInserted
		return nil
	}, nil
}

// TestFederationPull tests the federationPull() function.
func TestFederationPull(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	batchTime := time.Now().Truncate(time.Second)
	// Make all items have reasonable interval numbers based on test time.
	intervalNumber := publishmodel.IntervalNumber(batchTime.Add(-2 * 24 * time.Hour))
	aaa = setIntervalNumber(aaa, intervalNumber)
	bbb = setIntervalNumber(bbb, intervalNumber)
	ccc = setIntervalNumber(ccc, intervalNumber)
	ddd = setIntervalNumber(ddd, intervalNumber)

	cases := []struct {
		name           string
		batchSize      int
		fetchResponses []*federation.FederationFetchResponse
		wantExposures  []*publishmodel.Exposure
		wantState      *federation.FetchState
	}{
		{
			name: "no_results",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					PartialResponse: false,
					NextFetchState: &federation.FetchState{
						KeyCursor:        &federation.Cursor{},
						RevisedKeyCursor: &federation.Cursor{},
					},
				},
			},
			wantExposures: nil,
			wantState: &federation.FetchState{
				KeyCursor:        &federation.Cursor{},
				RevisedKeyCursor: &federation.Cursor{},
			},
		},
		{
			name: "basic_primary_results",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					Keys: []*federation.ExposureKey{
						setRegions(setReportType(copyExposureKey(aaa), federation.ExposureKey_CONFIRMED_TEST), "US"),
					},
					RevisedKeys:     []*federation.ExposureKey{},
					PartialResponse: false,
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							Timestamp: 100,
						},
					},
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, "confirmed", []string{"US"}, false, batchTime),
			},
			wantState: &federation.FetchState{
				KeyCursor: &federation.Cursor{
					Timestamp: 100,
				},
				RevisedKeyCursor: &federation.Cursor{},
			},
		},
		{
			name: "invalid_transmission_risk",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					Keys: []*federation.ExposureKey{
						setReportType(setRegions(setTransmissionRisk(copyExposureKey(aaa),
							verifyapi.TransmissionRiskConfirmedStandard),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
						// The next 2 keys should be dropped.
						setRegions(setTransmissionRisk(copyExposureKey(bbb), -1), "US"),
						setRegions(setTransmissionRisk(copyExposureKey(ccc), 9), "US"),
					},
					RevisedKeys:     []*federation.ExposureKey{},
					PartialResponse: false,
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							Timestamp: 100,
						},
					},
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, "confirmed", []string{"US"}, false, batchTime),
			},
			wantState: &federation.FetchState{
				KeyCursor: &federation.Cursor{
					Timestamp: 100,
				},
				RevisedKeyCursor: &federation.Cursor{},
			},
		},
		{
			name: "interval_number_too_old",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					Keys: []*federation.ExposureKey{
						setReportType(setRegions(setTransmissionRisk(copyExposureKey(aaa),
							verifyapi.TransmissionRiskConfirmedStandard),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
						setIntervalNumber(setReportType(setRegions(setTransmissionRisk(copyExposureKey(bbb),
							verifyapi.TransmissionRiskConfirmedStandard),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
							1), // TOO OLD - Key will be dropped
						setReportType(setRegions(setTransmissionRisk(copyExposureKey(ccc),
							verifyapi.TransmissionRiskConfirmedStandard),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
					},
					RevisedKeys:     []*federation.ExposureKey{},
					PartialResponse: false,
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							Timestamp: 100,
						},
					},
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, "confirmed", []string{"US"}, false, batchTime),
				makeRemoteExposure(ccc, "confirmed", []string{"US"}, false, batchTime),
			},
			wantState: &federation.FetchState{
				KeyCursor: &federation.Cursor{
					Timestamp: 100,
				},
				RevisedKeyCursor: &federation.Cursor{},
			},
		},
		{
			name: "partial_results",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					PartialResponse: true,
					Keys: []*federation.ExposureKey{
						setReportType(setRegions(copyExposureKey(aaa),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
						setReportType(setRegions(copyExposureKey(bbb),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
					},
					RevisedKeys: []*federation.ExposureKey{},
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							NextToken: "bbbb",
							Timestamp: 100,
						},
						RevisedKeyCursor: &federation.Cursor{},
					},
				},
				{
					PartialResponse: false,
					Keys: []*federation.ExposureKey{
						setReportType(setRegions(copyExposureKey(ccc),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
						setReportType(setRegions(copyExposureKey(ddd),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
					},
					RevisedKeys: []*federation.ExposureKey{},
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							NextToken: "",
							Timestamp: 200,
						},
						RevisedKeyCursor: &federation.Cursor{},
					},
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, "confirmed", []string{"US"}, false, batchTime),
				makeRemoteExposure(bbb, "confirmed", []string{"US"}, false, batchTime),
				makeRemoteExposure(ccc, "confirmed", []string{"US"}, false, batchTime),
				makeRemoteExposure(ddd, "confirmed", []string{"US"}, false, batchTime),
			},
			wantState: &federation.FetchState{
				KeyCursor: &federation.Cursor{
					Timestamp: 200,
				},
				RevisedKeyCursor: &federation.Cursor{},
			},
		},
		{
			name: "key_revision",
			fetchResponses: []*federation.FederationFetchResponse{
				{
					PartialResponse: false,
					Keys: []*federation.ExposureKey{
						setReportType(setRegions(copyExposureKey(aaa),
							"US"),
							federation.ExposureKey_CONFIRMED_CLINICAL_DIAGNOSIS),
					},
					RevisedKeys: []*federation.ExposureKey{
						setTransmissionRisk(setReportType(setRegions(copyExposureKey(aaa),
							"US"),
							federation.ExposureKey_CONFIRMED_TEST),
							verifyapi.TransmissionRiskConfirmedStandard),
					},
					NextFetchState: &federation.FetchState{
						KeyCursor: &federation.Cursor{
							Timestamp: 100,
						},
						RevisedKeyCursor: &federation.Cursor{
							Timestamp: 100,
						},
					},
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, "likely", []string{"US"}, false, batchTime),
				makeRemoteExposure(aaa, "confirmed", []string{"US"}, false, batchTime),
			},
			wantState: &federation.FetchState{
				KeyCursor: &federation.Cursor{
					Timestamp: 100,
				},
				RevisedKeyCursor: &federation.Cursor{
					Timestamp: 100,
				},
			},
		},
		/*
			{
				name:      "too large for batch",
				batchSize: 2,
				fetchResponses: []*federation.FederationFetchResponse{
					{
						Response: []*federation.ContactTracingResponse{
							{
								ContactTracingInfo: []*federation.ContactTracingInfo{
									{TransmissionRisk: 3, ExposureKeys: []*federation.ExposureKey{aaa, bbb}},
								},
								RegionIdentifiers: []string{"US"},
							},
							{
								ContactTracingInfo: []*federation.ContactTracingInfo{
									{TransmissionRisk: 2, ExposureKeys: []*federation.ExposureKey{ccc}},
								},
								RegionIdentifiers: []string{"US", "CA"},
							},
							{
								ContactTracingInfo: []*federation.ContactTracingInfo{
									{TransmissionRisk: 5, ExposureKeys: []*federation.ExposureKey{ddd}},
								},
								RegionIdentifiers: []string{"US"},
							},
						},
						FetchResponseKeyTimestamp: 400,
					},
				},
				wantExposures: []*publishmodel.Exposure{
					makeRemoteExposure(aaa, 3, "US"),
					makeRemoteExposure(bbb, 3, "US"),
					makeRemoteExposure(ccc, 2, "CA", "US"),
					makeRemoteExposure(ddd, 5, "US"),
				},
				wantTokens:       []string{""},
				wantMaxTimestamp: time.Unix(400, 0),
			},
		*/
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			query := &model.FederationInQuery{
				QueryID: queryID,
			}
			remote := remoteFetchServer{responses: tc.fetchResponses}
			idb := publishDB{}
			sdb := syncDB{
				maxTimestamp:        time.Unix(0, 0),
				maxRevisedTimestamp: time.Unix(0, 0),
			}
			batchStart := time.Now()
			if tc.batchSize > 0 {
				oldBatchSize := fetchBatchSize
				fetchBatchSize = tc.batchSize
				defer func() { fetchBatchSize = oldBatchSize }()
			}

			opts := pullOptions{
				deps: pullDependencies{
					fetch:               remote.fetch,
					insertExposures:     idb.insertExposures,
					startFederationSync: sdb.startFederationSync,
				},
				query:                        query,
				batchStart:                   batchStart,
				truncateWindow:               time.Hour,
				maxIntervalStartAge:          14 * 24 * time.Hour,
				maxMagnitudeSymptomOnsetDays: 14,
			}

			err := pull(ctx, &opts)
			if err != nil {
				t.Fatalf("pull returned err=%v, want err=nil", err)
			}

			if diff := cmp.Diff(tc.wantExposures, idb.exposures, cmpopts.IgnoreFields(publishmodel.Exposure{}, "CreatedAt"), cmpopts.IgnoreUnexported(publishmodel.Exposure{})); diff != "" {
				t.Errorf("exposures mismatch (-want +got):\n%s", diff)
			}
			if !sdb.syncStarted {
				t.Errorf("startFederatonSync not invoked")
			}
			if !sdb.syncCompleted {
				t.Errorf("startFederationSync completion callback not called")
			}
			if sdb.completed.Before(batchStart) {
				t.Errorf("federation sync ended too soon, completed: %v, batch started: %v", sdb.completed, batchStart)
			}
			if sdb.totalInserted != len(tc.wantExposures) {
				t.Errorf("federation sync total inserted got %d, want %d", sdb.totalInserted, len(tc.wantExposures))
			}
			if u := sdb.maxTimestamp.Unix(); u != tc.wantState.KeyCursor.Timestamp {
				t.Errorf("federation sync max timestamp got %v, want %v", u, tc.wantState.KeyCursor.Timestamp)
			}
			if u := sdb.maxRevisedTimestamp.Unix(); u != tc.wantState.RevisedKeyCursor.Timestamp {
				t.Errorf("federation sync max revised timestamp got %v, want %v", u, tc.wantState.RevisedKeyCursor.Timestamp)
			}
		})
	}
}

// makeRemoteExposure returns a mock publishmodel.Exposure with LocalProvenance=false.
func makeRemoteExposure(diagKey *federation.ExposureKey, reportType string, regions []string, traveler bool, createdAt time.Time) *publishmodel.Exposure {
	inf := makeExposure(diagKey, reportType, regions, traveler)
	inf.LocalProvenance = false
	inf.FederationSyncID = syncID
	inf.FederationQueryID = queryID
	inf.CreatedAt = time.Now()
	return inf
}

// makeExposure returns a mock model.Exposure.
func makeExposure(diagKey *federation.ExposureKey, reportType string, regions []string, traveler bool) *publishmodel.Exposure {
	return &publishmodel.Exposure{
		ExposureKey:      diagKey.ExposureKey,
		TransmissionRisk: publishmodel.ReportTypeTransmissionRisk(reportType, 0),
		Regions:          regions,
		Traveler:         traveler,
		IntervalNumber:   diagKey.IntervalNumber,
		IntervalCount:    144,
		CreatedAt:        time.Unix(int64(diagKey.IntervalNumber*100), 0), // Make unique from IntervalNumber.
		LocalProvenance:  true,
		ReportType:       reportType,
	}
}
