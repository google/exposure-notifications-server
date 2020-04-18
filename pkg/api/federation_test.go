package api

import (
	"cambio/pkg/database"
	"cambio/pkg/model"
	"cambio/pkg/pb"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

var (
	// listsAsSets are cmp.Options to remove list ordering from diffs of protos.
	listsAsSets = []cmp.Option{
		protocmp.Transform(),
		protocmp.SortRepeatedFields(&pb.FederationFetchResponse{}, "response"),
		protocmp.SortRepeatedFields(&pb.ContactTracingResponse{}, "contactTracingInfo"),
		protocmp.SortRepeatedFields(&pb.ContactTracingInfo{}, "diagnosisKeys"),
	}

	posver  = pb.DiagnosisStatus_positive_verified
	selfver = pb.DiagnosisStatus_self_reported

	aaa = &pb.DiagnosisKey{DiagnosisKey: []byte("aaa"), Timestamp: 1}
	bbb = &pb.DiagnosisKey{DiagnosisKey: []byte("bbb"), Timestamp: 2}
	ccc = &pb.DiagnosisKey{DiagnosisKey: []byte("ccc"), Timestamp: 3}
	ddd = &pb.DiagnosisKey{DiagnosisKey: []byte("ddd"), Timestamp: 4}
)

// makeInfection returns a mock model.Infection.
func makeInfection(diagKey *pb.DiagnosisKey, regions ...string) model.Infection {
	return model.Infection{
		Region: regions,
		// TODO(jasonco): Status: status,
		DiagnosisKey: diagKey.DiagnosisKey,
		KeyDay:       time.Unix(diagKey.Timestamp, 0),
		CreatedAt:    time.Unix(diagKey.Timestamp*100, 0), // Make unique from KeyDay.
	}
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

	case model.Infection:
		// Set the cursor equal to the most recent diagnosis key, suffixed with "_cursor".
		i.cursor = string(v.DiagnosisKey) + "_cursor"
		return &v, false, nil

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

// TestFetch tests the fetch() function.
func TestFetch(t *testing.T) {
	testCases := []struct {
		name           string
		excludeRegions []string
		iterations     []interface{}
		want           pb.FederationFetchResponse
	}{
		// TODO(jasonco): Add tests for diagnosis status.
		{
			name: "no results",
			want: pb.FederationFetchResponse{},
		},
		{
			name:       "basic results",
			iterations: []interface{}{makeInfection(aaa, "US"), makeInfection(bbb, "US"), makeInfection(ccc, "GB"), makeInfection(ddd, "US", "GB")},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{aaa, bbb}},
						},
					},
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name:           "exclude regions",
			excludeRegions: []string{"US", "CA"},
			iterations:     []interface{}{makeInfection(aaa, "US"), makeInfection(bbb, "CA"), makeInfection(ccc, "GB"), makeInfection(ddd, "US", "GB")},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"GB"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{ccc}},
						},
					},
					{
						RegionIdentifiers: []string{"GB", "US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{ddd}},
						},
					},
				},
				FetchResponseKeyTimestamp: 400,
			},
		},
		{
			name:           "exclude all regions",
			excludeRegions: []string{"US", "CA", "GB"},
			iterations:     []interface{}{makeInfection(aaa, "US"), makeInfection(bbb, "CA"), makeInfection(ccc, "GB"), makeInfection(ddd, "US", "CA", "GB")},
			want:           pb.FederationFetchResponse{},
		},
		{
			name:       "partial result",
			iterations: []interface{}{makeInfection(aaa, "US"), makeInfection(bbb, "CA"), timeout{}, makeInfection(ccc, "GB")},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{
						RegionIdentifiers: []string{"US"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{aaa}},
						},
					},
					{
						RegionIdentifiers: []string{"CA"},
						ContactTracingInfo: []*pb.ContactTracingInfo{
							{DiagnosisStatus: posver, DiagnosisKeys: []*pb.DiagnosisKey{bbb}},
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
			itFunc := func(ctx context.Context, criteria database.FetchInfectionsCriteria) (database.InfectionIterator, error) {
				return &testIterator{iterations: tc.iterations, cancel: cancel}, nil
			}

			got, err := server.fetch(ctx, &req, itFunc, time.Now())

			if err != nil {
				t.Fatalf("fetch() returned err=%v, want err=nil", err)
			}
			if diff := cmp.Diff(tc.want, got, listsAsSets...); diff != "" {
				t.Errorf("fetch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
