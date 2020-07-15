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
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"

	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/pb"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
)

var (
	syncID int64 = 999

	aaa = &pb.ExposureKey{ExposureKey: []byte("aaa"), IntervalNumber: 1}
	bbb = &pb.ExposureKey{ExposureKey: []byte("bbb"), IntervalNumber: 2}
	ccc = &pb.ExposureKey{ExposureKey: []byte("ccc"), IntervalNumber: 3}
	ddd = &pb.ExposureKey{ExposureKey: []byte("ddd"), IntervalNumber: 4}
)

// makeRemoteExposure returns a mock publishmodel.Exposure with LocalProvenance=false.
func makeRemoteExposure(diagKey *pb.ExposureKey, diagStatus int, regions ...string) *publishmodel.Exposure {
	inf := makeExposure(diagKey, diagStatus, regions...)
	inf.LocalProvenance = false
	inf.FederationSyncID = syncID
	return inf
}

// remoteFetchServer mocks responses from the remote federation server.
type remoteFetchServer struct {
	responses []*pb.FederationFetchResponse
	gotTokens []string
	index     int
}

func (r *remoteFetchServer) fetch(ctx context.Context, req *pb.FederationFetchRequest, opts ...grpc.CallOption) (*pb.FederationFetchResponse, error) {
	r.gotTokens = append(r.gotTokens, req.NextFetchToken)
	if r.responses == nil || r.index > len(r.responses) {
		return &pb.FederationFetchResponse{}, nil
	}
	response := r.responses[r.index]
	r.index++
	return response, nil
}

// publishDB mocks the database, recording exposure insertions.
type publishDB struct {
	exposures []*publishmodel.Exposure
}

func (idb *publishDB) insertExposures(ctx context.Context, exposures []*publishmodel.Exposure) (int, error) {
	idb.exposures = append(idb.exposures, exposures...)
	return len(exposures), nil
}

// syncDB mocks the database, recording start and complete invocations for a sync record.
type syncDB struct {
	syncStarted   bool
	syncCompleted bool
	completed     time.Time
	maxTimestamp  time.Time
	totalInserted int
}

func (sdb *syncDB) startFederationSync(ctx context.Context, query *model.FederationInQuery, start time.Time) (int64, database.FinalizeSyncFn, error) {
	sdb.syncStarted = true
	timerStart := time.Now()
	return syncID, func(maxTimestamp time.Time, totalInserted int) error {
		sdb.syncCompleted = true
		sdb.completed = start.Add(time.Since(timerStart))
		sdb.maxTimestamp = maxTimestamp
		sdb.totalInserted = totalInserted
		return nil
	}, nil
}

// TestFederationPull tests the federationPull() function.
func TestFederationPull(t *testing.T) {
	testCases := []struct {
		name             string
		batchSize        int
		fetchResponses   []*pb.FederationFetchResponse
		wantExposures    []*publishmodel.Exposure
		wantTokens       []string
		wantMaxTimestamp time.Time
	}{
		{
			name:             "no results",
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(0, 0),
		},
		{
			name: "basic results",
			fetchResponses: []*pb.FederationFetchResponse{
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 2, ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US", "CA"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 3, ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"US"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, 1, "US"),
				makeRemoteExposure(bbb, 1, "US"),
				makeRemoteExposure(ccc, 2, "CA", "US"),
				makeRemoteExposure(ddd, 3, "US"),
			},
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(400, 0),
		},
		{
			name: "invalid transmission risk",
			fetchResponses: []*pb.FederationFetchResponse{
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: -1, ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US", "CA"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 9, ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"US"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, 1, "US"),
				makeRemoteExposure(bbb, 1, "US"),
			},
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(400, 0),
		},
		{
			name: "partial results",
			fetchResponses: []*pb.FederationFetchResponse{
				{
					PartialResponse: true,
					NextFetchToken:  "abcdef",
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 8, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
					},
					FetchResponseKeyTimestamp: 200,
				},
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 7, ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 6, ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"CA"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantExposures: []*publishmodel.Exposure{
				makeRemoteExposure(aaa, 8, "US"),
				makeRemoteExposure(bbb, 8, "US"),
				makeRemoteExposure(ccc, 7, "US"),
				makeRemoteExposure(ddd, 6, "CA"),
			},
			wantTokens:       []string{"", "abcdef"},
			wantMaxTimestamp: time.Unix(400, 0),
		},
		{
			name:      "too large for batch",
			batchSize: 2,
			fetchResponses: []*pb.FederationFetchResponse{
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 3, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 2, ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US", "CA"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: 5, ExposureKeys: []*pb.ExposureKey{ddd}},
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			query := &model.FederationInQuery{}
			remote := remoteFetchServer{responses: tc.fetchResponses}
			idb := publishDB{}
			sdb := syncDB{}
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
				query:          query,
				batchStart:     batchStart,
				truncateWindow: time.Hour,
			}

			err := pull(ctx, metrics.NewLogsBasedFromContext(ctx), &opts)
			if err != nil {
				t.Fatalf("pull returned err=%v, want err=nil", err)
			}

			if diff := cmp.Diff(tc.wantExposures, idb.exposures, cmpopts.IgnoreFields(publishmodel.Exposure{}, "CreatedAt"), cmpopts.IgnoreUnexported(publishmodel.Exposure{})); diff != "" {
				t.Errorf("exposures mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.wantTokens, remote.gotTokens); diff != "" {
				t.Errorf("tokens mismatch (-want +got):\n%s", diff)
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
			if !sdb.maxTimestamp.Equal(tc.wantMaxTimestamp) {
				t.Errorf("federation sync max timestamp got %v, want %v", sdb.maxTimestamp, tc.wantMaxTimestamp)
			}
		})
	}
}

func makeExposure(diagKey *pb.ExposureKey, diagStatus int, regions ...string) *publishmodel.Exposure {
	return &publishmodel.Exposure{
		Regions:          regions,
		TransmissionRisk: diagStatus,
		ExposureKey:      diagKey.ExposureKey,
		IntervalNumber:   diagKey.IntervalNumber,
		CreatedAt:        time.Unix(int64(diagKey.IntervalNumber*100), 0), // Make unique from IntervalNumber.
		LocalProvenance:  true,
	}
}
