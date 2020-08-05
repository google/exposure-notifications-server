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
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/revision"
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

	testCases := []struct {
		name           string
		expectKeyCount int64
	}{
		{
			name:           "empty_db",
			expectKeyCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			keyID := "test" + t.Name()
			kms.AddEncryptionKey(keyID)
			config := &Config{
				RevisionToken: revision.Config{KeyID: keyID},
			}
			config.RevisionToken.KeyID = keyID
			server, err := NewServer(config, env)

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
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
