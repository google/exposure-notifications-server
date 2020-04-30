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
	"errors"
	"testing"
	"time"

	"github.com/googlepartners/exposure-notifications/internal/database"
	"github.com/googlepartners/exposure-notifications/internal/model"
	"github.com/googlepartners/exposure-notifications/internal/pb"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	// listsAsSets are cmp.Options to remove list ordering from diffs of protos.
	listsAsSets = []cmp.Option{
		protocmp.Transform(),
		protocmp.SortRepeatedFields(&pb.FederationFetchResponse{}, "response"),
		protocmp.SortRepeatedFields(&pb.ContactTracingResponse{}, "contactTracingInfo"),
		protocmp.SortRepeatedFields(&pb.ContactTracingInfo{}, "exposureKeys"),
	}

	posver  = pb.TransmissionRisk_positive_verified
	selfver = pb.TransmissionRisk_self_reported

	aaa = &pb.ExposureKey{ExposureKey: []byte("aaa"), IntervalNumber: 1}
	bbb = &pb.ExposureKey{ExposureKey: []byte("bbb"), IntervalNumber: 2}
	ccc = &pb.ExposureKey{ExposureKey: []byte("ccc"), IntervalNumber: 3}
	ddd = &pb.ExposureKey{ExposureKey: []byte("ddd"), IntervalNumber: 4}
)

// makeInfection returns a mock model.Infection.
func makeInfection(diagKey *pb.ExposureKey, diagStatus pb.TransmissionRisk, regions ...string) *model.Infection {
	return &model.Infection{
		Regions:          regions,
		TransmissionRisk: int(diagStatus),
		ExposureKey:      diagKey.ExposureKey,
		IntervalNumber:   diagKey.IntervalNumber,
		CreatedAt:        time.Unix(int64(diagKey.IntervalNumber*100), 0).UTC(), // Make unique from IntervalNumber.
		LocalProvenance:  true,
	}
}

func makeInfectionWithVerification(diagKey *pb.ExposureKey, diagStatus pb.TransmissionRisk, verificationAuthorityName string, regions ...string) *model.Infection {
	inf := makeInfection(diagKey, diagStatus, regions...)
	inf.VerificationAuthorityName = verificationAuthorityName
	return inf
}

// timeout is used by testIterator to indicate that a timeout signal should be sent.
type timeout struct{}

// testIterator is a mock database iterator. It can return a stream of model.Infections, or send a timeout signal.
type testIterator struct {
	iterations []interface{}
	cancel     context.CancelFunc
	index      int
	cursor     string
}

func (i *testIterator) Next() (*model.Infection, bool, error) {
	if i.index > len(i.iterations)-1 {
		// Reached end of results set.
		return nil, true, nil
	}

	item := i.iterations[i.index]
	i.index++

	// Switch on the type of iteration (either model.Infection or timeout).
	switch v := item.(type) {

	case *model.Infection:
		// Set the cursor equal to the most recent diagnosis key, suffixed with "_cursor".
		i.cursor = string(v.ExposureKey) + "_cursor"
		return v, false, nil

	case timeout:
		if i.cancel == nil {
			return nil, false, errors.New("timeout encountered, but no CancelFunc configured")
		}

		i.cancel()
		return nil, false, nil
	}

	return nil, false, errors.New("unexpected end of result set")
}

func (i *testIterator) Cursor() (string, error) {
	return i.cursor, nil
}

func (i *testIterator) Close() error {
	return nil
}

// TestFetch tests the fetch() function.
func TestFetch(t *testing.T) {
	testCases := []struct {
		name           string
		excludeRegions []string
		iterations     []interface{}
		want           pb.FederationFetchResponse
	}{
		{
			name: "no results",
			want: pb.FederationFetchResponse{},
		},
		{
			name: "basic results",
			iterations: []interface{}{
				makeInfection(aaa, posver, "US"),
				makeInfection(bbb, posver, "US"),
				makeInfection(ccc, posver, "GB"),
				makeInfection(ddd, posver, "US", "GB"),
			},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
						},
					},
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name: "results combined on status",
			iterations: []interface{}{
				makeInfection(aaa, posver, "US"),
				makeInfection(bbb, posver, "US"),
				makeInfection(ccc, selfver, "US"),
				makeInfection(ddd, selfver, "CA"),
			},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							{TransmissionRisk: selfver, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"CA"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: selfver, ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name: "results combined on status and verification",
			iterations: []interface{}{
				makeInfectionWithVerification(aaa, posver, "AAA", "US"),
				makeInfectionWithVerification(bbb, posver, "AAA", "US"),
				makeInfectionWithVerification(ccc, posver, "BBB", "US"),
				makeInfectionWithVerification(ddd, selfver, "AAA", "US"),
			},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, VerificationAuthorityName: "AAA", ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							{TransmissionRisk: posver, VerificationAuthorityName: "BBB", ExposureKeys: []*pb.ExposureKey{ccc}},
							{TransmissionRisk: selfver, VerificationAuthorityName: "AAA", ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name:           "exclude regions",
			excludeRegions: []string{"US", "CA"},
			iterations: []interface{}{
				makeInfection(aaa, posver, "US"),
				makeInfection(bbb, posver, "CA"),
				makeInfection(ccc, posver, "GB"),
				makeInfection(ddd, posver, "US", "GB"),
			},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name:           "exclude all regions",
			excludeRegions: []string{"US", "CA", "GB"},
			iterations: []interface{}{
				makeInfection(aaa, posver, "US"),
				makeInfection(bbb, posver, "CA"),
				makeInfection(ccc, posver, "GB"),
				makeInfection(ddd, posver, "US", "CA", "GB"),
			},
			want: pb.FederationFetchResponse{},
		},
		{
			name: "partial result",
			iterations: []interface{}{
				makeInfection(aaa, posver, "US"),
				makeInfection(bbb, posver, "CA"),
				timeout{},
				makeInfection(ccc, posver, "GB"),
			},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{aaa}},
						},
					},
					{
						RegionIdentifiers: []string{"CA"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: posver, ExposureKeys: []*pb.ExposureKey{bbb}},
						},
					},
				},
				PartialResponse:           true,
				FetchResponseKeyTimestamp: 200,
				NextFetchToken:            "bbb_cursor",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := federationServer{}
			req := pb.FederationFetchRequest{ExcludeRegionIdentifiers: tc.excludeRegions}
			ctx, cancel := context.WithCancel(context.Background())
			itFunc := func(ctx context.Context, criteria database.IterateInfectionsCriteria) (database.InfectionIterator, error) {
				return &testIterator{iterations: tc.iterations, cancel: cancel}, nil
			}

			got, err := server.fetch(ctx, &req, itFunc, time.Now().UTC())

			if err != nil {
				t.Fatalf("fetch() returned err=%v, want err=nil", err)
			}
			if diff := cmp.Diff(tc.want, got, listsAsSets...); diff != "" {
				t.Errorf("fetch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
