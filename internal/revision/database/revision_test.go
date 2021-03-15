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

package database

import (
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRevisionKey(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	cfg := KMSConfig{keyID, kms}
	revDB, err := New(testDB, &cfg)
	if err != nil {
		t.Fatalf("unable to provision revision DB: %v", err)
	}

	want, err := revDB.CreateRevisionKey(ctx)
	if err != nil {
		t.Fatalf("unable to create revision key: %v", err)
	}

	got, err := revDB.GetEffectiveRevisionKey(ctx)
	if err != nil {
		t.Fatalf("unable to read effective revision keyL: %v", err)
	}

	if diff := cmp.Diff(want, got, database.ApproxTime); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestMultipleRevisionKeys(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	cfg := KMSConfig{keyID, kms}
	revDB, err := New(testDB, &cfg)
	if err != nil {
		t.Fatalf("unable to provision revision DB: %v", err)
	}

	// Create two revision keys.
	key1, err := revDB.CreateRevisionKey(ctx)
	if err != nil {
		t.Fatalf("failed to create revision key: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	key2, err := revDB.CreateRevisionKey(ctx)
	if err != nil {
		t.Fatalf("failed to create revision key: %v", err)
	}

	// Check effective revision key (Should be second one)
	{
		got, err := revDB.GetEffectiveRevisionKey(ctx)
		if err != nil {
			t.Fatalf("unable to read effective keys: %v", err)
		}
		if diff := cmp.Diff(key2, got, database.ApproxTime); diff != "" {
			t.Fatalf("wrong effective key (-want, +got):\n%s", diff)
		}
	}

	sorter := cmpopts.SortSlices(
		func(a, b *RevisionKey) bool {
			return a.CreatedAt.Before(b.CreatedAt)
		})

	// Check that both keys are available.
	{
		gotID, got, err := revDB.GetAllowedRevisionKeys(ctx)
		if err != nil {
			t.Fatalf("unable to read all keys: %v", err)
		}
		want := []*RevisionKey{key1, key2}
		if diff := cmp.Diff(want, got, sorter, database.ApproxTime); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
		if gotID != key2.KeyID {
			t.Fatalf("wrong effective key ID want: %v got: %v", key2.KeyID, gotID)
		}
	}
	// And check that both keys are available in the ID check.
	{
		want := map[int64]struct{}{
			key1.KeyID: {},
			key2.KeyID: {},
		}
		gotID, got, err := revDB.GetAllowedRevisionKeyIDs(ctx)
		if err != nil {
			t.Fatalf("unable to get allowed key IDs: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
		if gotID != key2.KeyID {
			t.Fatalf("wrong effective key, want: %v got: %v", key2.KeyID, gotID)
		}
	}

	// Destroy key2
	if err := revDB.DestroyKey(ctx, key2.KeyID); err != nil {
		t.Fatalf("unable to detroy key: %v", err)
	}

	// Check that effective revision key has changed.
	{
		got, err := revDB.GetEffectiveRevisionKey(ctx)
		if err != nil {
			t.Fatalf("unable to read effective keys: %v", err)
		}
		if diff := cmp.Diff(key1, got, database.ApproxTime); diff != "" {
			t.Fatalf("wrong effective key (-want, +got):\n%s", diff)
		}
	}

	// Only key 2 should be available now.
	{
		gotID, got, err := revDB.GetAllowedRevisionKeys(ctx)
		if err != nil {
			t.Fatalf("unable to read all keys: %v", err)
		}
		want := []*RevisionKey{key1}
		if diff := cmp.Diff(want, got, sorter, database.ApproxTime); diff != "" {
			t.Fatalf("mismatch (-want, +got):\n%s", diff)
		}
		if gotID != key1.KeyID {
			t.Fatalf("wrong effective key ID: want: %v got: %v", key1.KeyID, gotID)
		}
	}
}
