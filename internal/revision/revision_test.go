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

package revision

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestBuildTokenBuffer(t *testing.T) {
	cases := []struct {
		name    string
		publish []*model.Exposure
		want    *pb.RevisionTokenData
	}{
		{
			name: "one",
			publish: []*model.Exposure{
				{
					ExposureKey:    []byte{1, 2, 3, 4},
					IntervalNumber: 200000,
					IntervalCount:  144,
				},
			},
			want: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
					{
						TemporaryExposureKey: []byte{1, 2, 3, 4},
						IntervalNumber:       200000,
						IntervalCount:        144,
					},
				},
			},
		},
		{
			name: "wrong_order",
			publish: []*model.Exposure{
				{
					ExposureKey:    []byte{4, 3, 2, 1},
					IntervalNumber: 200144,
					IntervalCount:  144,
				},
				{
					ExposureKey:    []byte{1, 2, 3, 4},
					IntervalNumber: 200000,
					IntervalCount:  144,
				},
			},
			want: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
					{
						TemporaryExposureKey: []byte{1, 2, 3, 4},
						IntervalNumber:       200000,
						IntervalCount:        144,
					},
					{
						TemporaryExposureKey: []byte{4, 3, 2, 1},
						IntervalNumber:       200144,
						IntervalCount:        144,
					},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildTokenBufer(tc.publish)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("unable to generate AES key: %v", err)
	}

	// Build data to serialize and encrypt.
	source := []*model.Exposure{
		{
			ExposureKey:    []byte{1, 2, 3, 4},
			IntervalNumber: 254321,
			IntervalCount:  144,
		},
		{
			ExposureKey:    []byte{5, 6, 7, 8},
			IntervalNumber: 254465,
			IntervalCount:  144,
		},
	}
	aad := []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	kid := "123"
	// Expected result in the end
	want := &pb.RevisionTokenData{
		RevisableKeys: []*pb.RevisableKey{
			{
				TemporaryExposureKey: []byte{1, 2, 3, 4},
				IntervalNumber:       254321,
				IntervalCount:        144,
			},
			{
				TemporaryExposureKey: []byte{5, 6, 7, 8},
				IntervalNumber:       254465,
				IntervalCount:        144,
			},
		},
	}

	encrypted, err := MakeRevisionToken(source, aad, kid, key)
	if err != nil {
		t.Fatalf("error encrypting and serializing data: %v", err)
	}

	validKeys := map[string][]byte{
		kid: key,
	}
	got, err := UnmarshalRevisionToken(encrypted, aad, validKeys)
	if err != nil {
		t.Fatalf("error decrypting token: %v", err)
	}

	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
