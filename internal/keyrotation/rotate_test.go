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
	testDB := database.NewTestDatabase(t)
	kms, _ := keys.NewInMemory(context.Background())
	env := serverenv.New(ctx, serverenv.WithKeyManager(kms), serverenv.WithDatabase(testDB))
	keyID := "testKeyID"
	kms.AddEncryptionKey(keyID)
	config := &Config{
		RevisionToken:      revision.Config{KeyID: keyID},
		DeleteOldKeyPeriod: 14 * 24 * time.Hour, // two weeks
		NewKeyPeriod:       24 * time.Hour,      // one day
	}
	config.RevisionToken.KeyID = keyID
	server, err := NewServer(config, env)
	if err != nil {
		t.Fatalf("got unexpected error: %v", err)
	}

	// Wrap the key using the configured KMS.
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Errorf("unable to generate AES key: %w", err)
	}
	aad := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, aad); err != nil {
		t.Errorf("unable to generate random data: %w", err)
	}

	wrapped, err := kms.Encrypt(ctx, keyID, key, aad)
	if err != nil {
		t.Fatal(err)
	}

	neg20Days, _ := time.ParseDuration("-20d")
	staleTime := time.Now().Add(neg20Days)
	//notStaleTime := time.Now()

	testCases := []struct {
		name           string
		expectKeyCount int64
		keys           []revisiondb.RevisionKey
	}{
		{
			name:           "empty_db, create key",
			expectKeyCount: 1,
		},
		{
			name:           "four keys, drop two",
			expectKeyCount: 4,
			keys: []revisiondb.RevisionKey{
				revisiondb.RevisionKey{
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				revisiondb.RevisionKey{
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				revisiondb.RevisionKey{
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
				revisiondb.RevisionKey{
					CreatedAt:     staleTime,
					Allowed:       true,
					AAD:           aad,
					WrappedCipher: wrapped,
					DEK:           key,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range tc.keys {
				if err := insertRawKey(ctx, t, testDB, &k); err != nil {
					t.Error("Failed to insert keys: ", err)
				}
			}

			if err := server.doRotate(ctx); err != nil {
				t.Fatalf("doRotate failed: %v", err)
			}

			count, err := purgeAllKeys(ctx, t, testDB)
			if err != nil {
				t.Error("Failed to purge keys", err)
			}

			if count != tc.expectKeyCount {
				t.Errorf("Purged %d keys, wanted %d", count, tc.expectKeyCount)
			}
		})
	}
}

func purgeAllKeys(ctx context.Context, t *testing.T, db *database.DB) (int64, error) {
	t.Helper()
	var count int64 = 0
	if err := db.InTx(ctx, pgx.Serializable, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM RevisionKeys`)
		count = result.RowsAffected()
		return err
	}); err != nil {
		return count, fmt.Errorf("unable to clear keys: %w", err)
	}
	return count, nil
}

func insertRawKey(ctx context.Context, t *testing.T, db *database.DB, key *revisiondb.RevisionKey) error {
	if err := db.InTx(ctx, pgx.ReadCommitted, func(tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO
				RevisionKeys
				(aad, wrapped_cipher, created_at, allowed)
			VALUES
				($1, $2, $3, $4)
			RETURNING kid`,
			key.AAD, key.WrappedCipher, key.CreatedAt, true)
		if err := row.Scan(&key.KeyID); err != nil {
			return fmt.Errorf("fetching kid: %w", err)
		}
		t.Log("inserted key with Id", key.KeyID)
		return nil
	}); err != nil {
		return fmt.Errorf("unable to persist revision key: %w", err)
	}

	return nil
}
