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

// Package keyrotation implements the API handlers for running key rotation jobs.
package keyrotation

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/revision"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/jackc/pgx/v4"
)

func TestRotateKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kms := keys.TestKeyManager(t)
	keyID := keys.TestEncryptionKey(t, kms)

	config := &Config{
		RevisionToken:      revision.Config{KeyID: keyID},
		DeleteOldKeyPeriod: 14 * 24 * time.Hour, // two weeks
		NewKeyPeriod:       24 * time.Hour,      // one day
	}
	key, aad, wrapped := testMakeKey(ctx, t, kms, keyID)

	neg20Days, _ := time.ParseDuration("-480h")
	neg40Days, _ := time.ParseDuration("-960h")
	staleTime := time.Now().Add(neg20Days)
	reallyStaleTime := time.Now().Add(neg40Days)
	notStaleTime := time.Now()

	testCases := []struct {
		name             string
		expectKeyCount   int
		expectAllowed    []int64
		expectNotAllowed []int64
		keys             []revisiondb.RevisionKey
	}{
		{
			name:           "empty_db, create key",
			expectKeyCount: 1,
		},
		{
			name:           "already fresh",
			expectKeyCount: 1,
			expectAllowed:  []int64{100},
			keys: []revisiondb.RevisionKey{
				{
					KeyID:         100,
					CreatedAt:     notStaleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
			},
		},
		{
			name:           "dont delete old effective",
			expectKeyCount: 2, // should generate a new one
			expectAllowed:  []int64{111},
			keys: []revisiondb.RevisionKey{
				{
					KeyID:         111,
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
			},
		},
		{
			name:           "one fresh, one stale",
			expectKeyCount: 2,
			expectAllowed:  []int64{121, 122},
			keys: []revisiondb.RevisionKey{
				{
					KeyID:         121,
					CreatedAt:     notStaleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				{
					KeyID:         122,
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
			},
		},
		{
			name:             "very old retired",
			expectKeyCount:   2,
			expectAllowed:    []int64{131, 132},
			expectNotAllowed: []int64{133},
			keys: []revisiondb.RevisionKey{
				{
					KeyID:         131,
					CreatedAt:     notStaleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				{
					KeyID:         132,
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				{
					KeyID:         133,
					CreatedAt:     reallyStaleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testDB := database.NewTestDatabase(t)
			env := serverenv.New(ctx, serverenv.WithKeyManager(kms), serverenv.WithDatabase(testDB))
			server, err := NewServer(config, env)
			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}

			for _, k := range tc.keys {
				if err := testInsertRawKey(ctx, t, testDB, &k); err != nil {
					t.Error("Failed to insert keys: ", err)
				}
			}

			if err := server.doRotate(ctx); err != nil {
				t.Fatalf("doRotate failed: %v", err)
			}

			_, keys, err := server.revisionDB.GetAllowedRevisionKeyIDs(ctx)
			if err != nil {
				t.Fatalf("GetAllowedRevisionKeyIDs failed: %v", err)
			}

			if len(keys) != tc.expectKeyCount {
				t.Errorf("Allowed keys %d keys, wanted %d", len(keys), tc.expectKeyCount)
			}

			for _, i := range tc.expectAllowed {
				if _, has := keys[i]; !has {
					t.Errorf("Expected id %d to be allowed", i)
				}
			}

			for _, i := range tc.expectNotAllowed {
				if _, has := keys[i]; has {
					t.Errorf("Expected id %d to be NOT allowed", i)
				}
			}
		})
	}
}

func testMakeKey(ctx context.Context, t testing.TB, kms keys.KeyManager, keyID string) (key []byte, aad []byte, wrapped []byte) {
	key = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Errorf("unable to generate AES key: %w", err)
	}
	aad = make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, aad); err != nil {
		t.Errorf("unable to generate random data: %w", err)
	}
	wrapped, err := kms.Encrypt(ctx, keyID, key, aad)
	if err != nil {
		t.Fatal(err)
	}

	return
}

func testInsertRawKey(ctx context.Context, t testing.TB, db *database.DB, key *revisiondb.RevisionKey) error {
	t.Helper()
	if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO
				RevisionKeys
				(kid, aad, wrapped_cipher, created_at, allowed)
			VALUES
				($1, $2, $3, $4, $5)
			RETURNING kid`,
			key.KeyID, key.AAD, key.WrappedCipher, key.CreatedAt, true)
		return err
	}); err != nil {
		return fmt.Errorf("unable to persist revision key: %w", err)
	}

	t.Log("inserted key with Id", key.KeyID)
	return nil
}
