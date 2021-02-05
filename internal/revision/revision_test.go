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
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/pb"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testDatabaseInstance *database.TestInstance

func TestMain(m *testing.M) {
	testDatabaseInstance = database.MustTestInstance()
	defer testDatabaseInstance.MustClose()
	m.Run()
}

func TestBuildTokenBuffer(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		publish  []*model.Exposure
		previous *pb.RevisionTokenData
		want     *pb.RevisionTokenData
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
			previous: nil,
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
			name: "merge_previous",
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
			previous: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
					{
						TemporaryExposureKey: []byte{4, 3, 2, 1},
						IntervalNumber:       200144,
						IntervalCount:        144,
					},
					{
						TemporaryExposureKey: []byte{8, 9, 10, 11},
						IntervalNumber:       200144,
						IntervalCount:        82,
					},
				},
			},
			want: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
					{
						TemporaryExposureKey: []byte{4, 3, 2, 1},
						IntervalNumber:       200144,
						IntervalCount:        144,
					},
					{
						TemporaryExposureKey: []byte{8, 9, 10, 11},
						IntervalNumber:       200144,
						IntervalCount:        82,
					},
					{
						TemporaryExposureKey: []byte{1, 2, 3, 4},
						IntervalNumber:       200000,
						IntervalCount:        144,
					},
				},
			},
		},
		{
			name: "merge_duplicates_prefers_previous",
			publish: []*model.Exposure{
				{
					ExposureKey:    []byte{4, 3, 2, 1},
					IntervalNumber: 200134,
					IntervalCount:  122,
				},
			},
			previous: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
					{
						TemporaryExposureKey: []byte{4, 3, 2, 1},
						IntervalNumber:       200144,
						IntervalCount:        144,
					},
				},
			},
			want: &pb.RevisionTokenData{
				RevisableKeys: []*pb.RevisableKey{
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
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildTokenBufer(tc.previous, tc.publish)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	cfg := revisiondb.KMSConfig{
		WrapperKeyID: keyID,
		KeyManager:   kms,
	}
	revDB, err := revisiondb.New(testDB, &cfg)
	if err != nil {
		t.Fatalf("unable to provision revision DB: %v", err)
	}

	// Add an allowed key.
	firstKey, err := revDB.CreateRevisionKey(ctx)
	if err != nil {
		t.Fatalf("unable to create a revision key: %v", err)
	}

	tm, err := New(ctx, revDB, time.Second, 28)
	if err != nil {
		t.Fatalf("unable to build token manager: %v", err)
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

	encrypted, err := tm.MakeRevisionToken(ctx, nil, source, aad)
	if err != nil {
		t.Fatalf("error encrypting and serializing data: %v", err)
	}

	got, err := tm.UnmarshalRevisionToken(ctx, encrypted, aad)
	if err != nil {
		t.Fatalf("error decrypting token: %v", err)
	}

	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	// Replace the effective key and destroy the original.
	if _, err := revDB.CreateRevisionKey(ctx); err != nil {
		t.Fatalf("unable to create second revision key: %v", err)
	}
	if err := revDB.DestroyKey(ctx, firstKey.KeyID); err != nil {
		t.Fatalf("unable to destroy revision key: %v", err)
	}

	// force expire the cache.
	tm.cacheRefreshAfter = time.Now().Add(-1 * time.Second)

	// Decryption of the key should fail this time.
	wantErr := fmt.Sprintf("token has invalid key id: %v", firstKey.KeyID)
	if _, err := tm.UnmarshalRevisionToken(ctx, encrypted, aad); err == nil || err.Error() != wantErr {
		t.Fatalf("want error: %v, got: %v", wantErr, err)
	}
}

func TestExpandRevisionToken(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	cfg := revisiondb.KMSConfig{
		WrapperKeyID: keyID,
		KeyManager:   kms,
	}
	revDB, err := revisiondb.New(testDB, &cfg)
	if err != nil {
		t.Fatalf("unable to provision revision DB: %v", err)
	}

	// Add an allowed key.
	_, err = revDB.CreateRevisionKey(ctx)
	if err != nil {
		t.Fatalf("unable to create a revision key: %v", err)
	}

	tm, err := New(ctx, revDB, time.Second, 28)
	if err != nil {
		t.Fatalf("unable to build token manager: %v", err)
	}

	// Create a token where there is a previous but no new keys.
	previousToken := &pb.RevisionTokenData{
		RevisableKeys: []*pb.RevisableKey{
			{
				TemporaryExposureKey: []byte{1, 2, 3, 4},
				IntervalNumber:       254321,
				IntervalCount:        144,
			},
		},
	}
	source := []*model.Exposure{}
	aad := []byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	// Expected result in the end
	want := &pb.RevisionTokenData{
		RevisableKeys: []*pb.RevisableKey{
			{
				TemporaryExposureKey: []byte{1, 2, 3, 4},
				IntervalNumber:       254321,
				IntervalCount:        144,
			},
		},
	}

	encrypted, err := tm.MakeRevisionToken(ctx, previousToken, source, aad)
	if err != nil {
		t.Fatalf("error encrypting and serializing data: %v", err)
	}

	got, err := tm.UnmarshalRevisionToken(ctx, encrypted, aad)
	if err != nil {
		t.Fatalf("error decrypting token: %v", err)
	}

	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	// Create it again, this time merging in new keys.
	source = []*model.Exposure{
		{
			ExposureKey:    []byte{5, 6, 7, 8},
			IntervalNumber: 254465,
			IntervalCount:  144,
		},
	}
	want = &pb.RevisionTokenData{
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

	encrypted, err = tm.MakeRevisionToken(ctx, previousToken, source, aad)
	if err != nil {
		t.Fatalf("error encrypting and serializing data: %v", err)
	}

	got, err = tm.UnmarshalRevisionToken(ctx, encrypted, aad)
	if err != nil {
		t.Fatalf("error decrypting token: %v", err)
	}

	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(pb.RevisionTokenData{}), cmpopts.IgnoreUnexported(pb.RevisableKey{})); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
