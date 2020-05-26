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

	"github.com/google/exposure-notifications-server/internal/publish/database"

	"github.com/google/exposure-notifications-server/internal/publish/model"

	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"google.golang.org/grpc/metadata"

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

	aaa = &pb.ExposureKey{ExposureKey: []byte("aaa"), IntervalNumber: 1}
	bbb = &pb.ExposureKey{ExposureKey: []byte("bbb"), IntervalNumber: 2}
	ccc = &pb.ExposureKey{ExposureKey: []byte("ccc"), IntervalNumber: 3}
	ddd = &pb.ExposureKey{ExposureKey: []byte("ddd"), IntervalNumber: 4}
)

// makeExposure returns a mock model.Exposure.
func makeExposure(diagKey *pb.ExposureKey, diagStatus int, regions ...string) *model.Exposure {
	return &model.Exposure{
		Regions:          regions,
		TransmissionRisk: diagStatus,
		ExposureKey:      diagKey.ExposureKey,
		IntervalNumber:   diagKey.IntervalNumber,
		CreatedAt:        time.Unix(int64(diagKey.IntervalNumber*100), 0), // Make unique from IntervalNumber.
		LocalProvenance:  true,
	}
}

// timeout is used by testIterator to indicate that a timeout signal should be sent.
type timeout struct{}

func iterFunc(elements []interface{}) iterateExposuresFunc {
	return func(_ context.Context, _ database.IterateExposuresCriteria, f func(*model.Exposure) error) (string, error) {
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
}

// TestFetch tests the fetch() function.
func TestFetch(t *testing.T) {
	testCases := []struct {
		name           string
		excludeRegions []string
		iterations     []interface{}
		want           *pb.FederationFetchResponse
	}{
		{
			name: "no results",
			want: &pb.FederationFetchResponse{},
		},
		{
			name: "basic results",
			iterations: []interface{}{
				makeExposure(aaa, 1, "US"),
				makeExposure(bbb, 1, "US"),
				makeExposure(ccc, 3, "GB"),
				makeExposure(ddd, 4, "US", "GB"),
			},
			want: &pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
						},
					},
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 3, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 4, ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name: "results combined on status",
			iterations: []interface{}{
				makeExposure(aaa, 8, "US"),
				makeExposure(bbb, 8, "US"),
				makeExposure(ccc, 6, "US"),
				makeExposure(ddd, 5, "CA"),
			},
			want: &pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 8, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							{TransmissionRisk: 6, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"CA"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 5, ExposureKeys: []*pb.ExposureKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name: "results combined on status and verification",
			iterations: []interface{}{
				makeExposure(aaa, 1, "US"),
				makeExposure(bbb, 1, "US"),
				makeExposure(ccc, 2, "US"),
				makeExposure(ddd, 3, "US"),
			},
			want: &pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{aaa, bbb}},
							{TransmissionRisk: 2, ExposureKeys: []*pb.ExposureKey{ccc}},
							{TransmissionRisk: 3, ExposureKeys: []*pb.ExposureKey{ddd}},
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
				makeExposure(aaa, 8, "US"),
				makeExposure(bbb, 8, "CA"),
				makeExposure(ccc, 2, "GB"),
				makeExposure(ddd, 1, "US", "GB"),
			},
			want: &pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 2, ExposureKeys: []*pb.ExposureKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{ddd}},
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
				makeExposure(aaa, 1, "US"),
				makeExposure(bbb, 1, "CA"),
				makeExposure(ccc, 1, "GB"),
				makeExposure(ddd, 1, "US", "CA", "GB"),
			},
			want: &pb.FederationFetchResponse{},
		},
		{
			name: "partial result",
			iterations: []interface{}{
				makeExposure(aaa, 1, "US"),
				makeExposure(bbb, 2, "CA"),
				timeout{},
				makeExposure(ccc, 3, "GB"),
			},
			want: &pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 1, ExposureKeys: []*pb.ExposureKey{aaa}},
						},
					},
					{
						RegionIdentifiers: []string{"CA"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{TransmissionRisk: 2, ExposureKeys: []*pb.ExposureKey{bbb}},
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
			ctx := context.Background()
			env := serverenv.New(ctx)
			server := Server{env: env}
			req := &pb.FederationFetchRequest{ExcludeRegionIdentifiers: tc.excludeRegions}
			got, err := server.fetch(context.Background(), req, iterFunc(tc.iterations), time.Now())
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
	want := "Abc123"
	md := metadata.New(map[string]string{"authorization": fmt.Sprintf("Bearer %s", want)})
	ctx := metadata.NewIncomingContext(context.Background(), md)

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
			ctx := metadata.NewIncomingContext(context.Background(), tc.mdMap)
			_, gotErr := rawToken(ctx)
			if gotErr == nil {
				t.Errorf("missing error response")
			}
		})
	}
}

// TestIntersect tests intersect().
func TestIntersect(t *testing.T) {
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
