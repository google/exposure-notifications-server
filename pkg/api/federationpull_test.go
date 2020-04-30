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

package api

import (
	"context"
	"testing"
	"time"

	"github.com/googlepartners/exposure-notifications/pkg/database"
	"github.com/googlepartners/exposure-notifications/pkg/model"
	"github.com/googlepartners/exposure-notifications/pkg/pb"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc"
)

var (
	syncID = "ABC123"
)

// makeRemoteInfection returns a mock model.Infection with LocalProvenance=false.
func makeRemoteInfection(diagKey *pb.ExposureKey, diagStatus pb.TransmissionRisk, verificationAuthorityName string, regions ...string) *model.Infection {
	inf := makeInfectionWithVerification(diagKey, diagStatus, verificationAuthorityName, regions...)
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

// infectionDB mocks the database, recording infection insertions.
type infectionDB struct {
	infections []*model.Infection
}

func (idb *infectionDB) insertInfections(ctx context.Context, infections []*model.Infection) error {
	idb.infections = append(idb.infections, infections...)
	return nil
}

// syncDB mocks the database, recording start and complete invocations for a sync record.
type syncDB struct {
	syncStarted   bool
	syncCompleted bool
	completed     time.Time
	maxTimestamp  time.Time
	totalInserted int
}

func (sdb *syncDB) startFederationSync(ctx context.Context, query *model.FederationQuery, start time.Time) (string, database.FinalizeSyncFn, error) {
	sdb.syncStarted = true
	timerStart := time.Now().UTC()
	return syncID, func(maxTimestamp time.Time, totalInserted int) error {
		sdb.syncCompleted = true
		sdb.completed = start.Add(time.Now().UTC().Sub(timerStart))
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
		wantInfections   []*model.Infection
		wantTokens       []string
		wantMaxTimestamp time.Time
	}{
		{
			name:             "no results",
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(0, 0).UTC(),
		},
		{
			name: "basic results",
			fetchResponses: []*pb.FederationFetchResponse{
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: posver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: posver, VerificationAuthorityName: "AAA", ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US", "CA"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: selfver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"US"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantInfections: []*model.Infection{
				makeRemoteInfection(aaa, posver, "", "US"),
				makeRemoteInfection(bbb, posver, "", "US"),
				makeRemoteInfection(ccc, posver, "AAA", "CA", "US"),
				makeRemoteInfection(ddd, selfver, "", "US"),
			},
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(400, 0).UTC(),
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
								{TransmissionRisk: posver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
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
								{TransmissionRisk: posver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: selfver, VerificationAuthorityName: "AAA", ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"CA"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantInfections: []*model.Infection{
				makeRemoteInfection(aaa, posver, "", "US"),
				makeRemoteInfection(bbb, posver, "", "US"),
				makeRemoteInfection(ccc, posver, "", "US"),
				makeRemoteInfection(ddd, selfver, "AAA", "CA"),
			},
			wantTokens:       []string{"", "abcdef"},
			wantMaxTimestamp: time.Unix(400, 0).UTC(),
		},
		{
			name:      "too large for batch",
			batchSize: 2,
			fetchResponses: []*pb.FederationFetchResponse{
				{
					Response: []*pb.ContactTracingResponse{
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: posver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							},
							RegionIdentifiers: []string{"US"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: posver, VerificationAuthorityName: "AAA", ExposureKeys: []*pb.ExposureKey{ccc}},
							},
							RegionIdentifiers: []string{"US", "CA"},
						},
						{
							ContactTracingInfo: []*pb.ContactTracingInfo{
								{TransmissionRisk: selfver, VerificationAuthorityName: "", ExposureKeys: []*pb.ExposureKey{ddd}},
							},
							RegionIdentifiers: []string{"US"},
						},
					},
					FetchResponseKeyTimestamp: 400,
				},
			},
			wantInfections: []*model.Infection{
				makeRemoteInfection(aaa, posver, "", "US"),
				makeRemoteInfection(bbb, posver, "", "US"),
				makeRemoteInfection(ccc, posver, "AAA", "CA", "US"),
				makeRemoteInfection(ddd, selfver, "", "US"),
			},
			wantTokens:       []string{""},
			wantMaxTimestamp: time.Unix(400, 0).UTC(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			query := &model.FederationQuery{}
			remote := remoteFetchServer{responses: tc.fetchResponses}
			idb := infectionDB{}
			sdb := syncDB{}
			batchStart := time.Now().UTC()
			if tc.batchSize > 0 {
				oldBatchSize := fetchBatchSize
				fetchBatchSize = tc.batchSize
				defer func() { fetchBatchSize = oldBatchSize }()
			}
			deps := pullDependencies{
				fetch:               remote.fetch,
				insertInfections:    idb.insertInfections,
				startFederationSync: sdb.startFederationSync,
			}

			err := federationPull(ctx, deps, query, batchStart)
			if err != nil {
				t.Fatalf("pull returned err=%v, want err=nil", err)
			}

			if diff := cmp.Diff(tc.wantInfections, idb.infections, cmpopts.IgnoreFields(model.Infection{}, "CreatedAt")); diff != "" {
				t.Errorf("infections mismatch (-want +got):\n%s", diff)
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
			if sdb.totalInserted != len(tc.wantInfections) {
				t.Errorf("federation sync total inserted got %d, want %d", sdb.totalInserted, len(tc.wantInfections))
			}
			if sdb.maxTimestamp != tc.wantMaxTimestamp {
				t.Errorf("federation sync max timestamp got %v, want %v", sdb.maxTimestamp, tc.wantMaxTimestamp)
			}
		})
	}
}
