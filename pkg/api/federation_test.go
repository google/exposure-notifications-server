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
		protocmp.SortRepeatedFields(&pb.ContactTracingResponse{}, "diagnosisKeys"),
	}

	posver  = pb.DiagnosisStatus_positive_verified
	selfver = pb.DiagnosisStatus_self_reported

	aaa = &pb.DiagnosisKey{DiagnosisKey: []byte("aaa"), Timestamp: 1}
	bbb = &pb.DiagnosisKey{DiagnosisKey: []byte("bbb"), Timestamp: 2}
	ccc = &pb.DiagnosisKey{DiagnosisKey: []byte("ccc"), Timestamp: 3}
	ddd = &pb.DiagnosisKey{DiagnosisKey: []byte("ddd"), Timestamp: 4}
)

// TestConvertResponse tests convertResponse().
func TestConvertResponse(t *testing.T) {
	testCases := []struct {
		name string
		c    collator
		want []*pb.ContactTracingResponse
	}{
		{
			name: "empty",
			c:    collator{},
			want: []*pb.ContactTracingResponse{},
		},
		{
			name: "one record",
			c: collator{
				"US": diagKeys{posver: diagKeyList{aaa}},
			},
			want: []*pb.ContactTracingResponse{
				{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa}},
			},
		},
		{
			name: "same country same status",
			c: collator{
				"US": diagKeys{posver: diagKeyList{aaa, bbb}},
			},
			want: []*pb.ContactTracingResponse{
				{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa, bbb}},
			},
		},
		{
			name: "same country different status",
			c: collator{
				"US": diagKeys{posver: diagKeyList{aaa}, selfver: diagKeyList{bbb}},
			},
			want: []*pb.ContactTracingResponse{
				{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa}},
				{DiagnosisStatus: selfver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{bbb}},
			},
		},
		{
			name: "different country same status",
			c: collator{
				"US": diagKeys{posver: diagKeyList{aaa}},
				"CA": diagKeys{posver: diagKeyList{bbb}},
			},
			want: []*pb.ContactTracingResponse{
				{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa}},
				{DiagnosisStatus: posver, RegionIdentifier: "CA", DiagnosisKeys: diagKeyList{bbb}},
			},
		},
		{
			name: "different country different status",
			c: collator{
				"US": diagKeys{
					posver:  diagKeyList{aaa},
					selfver: diagKeyList{bbb},
				},
				"CA": diagKeys{
					posver:  diagKeyList{ccc},
					selfver: diagKeyList{ddd},
				},
			},
			want: []*pb.ContactTracingResponse{
				{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa}},
				{DiagnosisStatus: selfver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{bbb}},
				{DiagnosisStatus: posver, RegionIdentifier: "CA", DiagnosisKeys: diagKeyList{ccc}},
				{DiagnosisStatus: selfver, RegionIdentifier: "CA", DiagnosisKeys: diagKeyList{ddd}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			want := pb.FederationFetchResponse{Response: tc.want}

			got := pb.FederationFetchResponse{Response: convertResponse(tc.c)}

			if diff := cmp.Diff(want, got, listsAsSets...); diff != "" {
				t.Errorf("convertReponse() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// makeInfection returns a mock model.Infection.
func makeInfection(region string, diagKey *pb.DiagnosisKey) model.Infection {
	return model.Infection{
		Country: region,
		// TODO(jasonco): Status: status,
		DiagnosisKey: diagKey.DiagnosisKey,
		KeyDay:       time.Unix(diagKey.Timestamp, 0),
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
		{
			name: "no results",
			want: pb.FederationFetchResponse{},
		},
		{
			name:       "basic results",
			iterations: []interface{}{makeInfection("US", aaa), makeInfection("US", bbb)},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa, bbb}},
				},
				FetchResponseKeyTimestamp: 2,
			},
		},
		{
			name:           "exclude countries",
			excludeRegions: []string{"US", "CA"},
			iterations:     []interface{}{makeInfection("US", aaa), makeInfection("CA", bbb), makeInfection("GB", ccc)},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{DiagnosisStatus: posver, RegionIdentifier: "GB", DiagnosisKeys: diagKeyList{ccc}},
				},
				FetchResponseKeyTimestamp: 3,
			},
		},
		{
			name:           "exclude all countries",
			excludeRegions: []string{"US", "CA", "GB"},
			iterations:     []interface{}{makeInfection("US", aaa), makeInfection("CA", bbb), makeInfection("GB", ccc)},
			want:           pb.FederationFetchResponse{},
		},
		{
			name:       "partial result",
			iterations: []interface{}{makeInfection("US", aaa), makeInfection("CA", bbb), timeout{}, makeInfection("GB", ccc)},
			want: pb.FederationFetchResponse{
				Response: []*pb.ContactTracingResponse{
					{DiagnosisStatus: posver, RegionIdentifier: "US", DiagnosisKeys: diagKeyList{aaa}},
					{DiagnosisStatus: posver, RegionIdentifier: "CA", DiagnosisKeys: diagKeyList{bbb}},
				},
				PartialResponse:           true,
				FetchResponseKeyTimestamp: 2,
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

			got, err := server.fetch(ctx, &req, itFunc)

			if err != nil {
				t.Fatalf("fetch() returned err=%v, want err=nil", err)
			}
			if diff := cmp.Diff(tc.want, got, listsAsSets...); diff != "" {
				t.Errorf("fetch() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
