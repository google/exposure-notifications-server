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
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/revision"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

func TestRotateKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB := database.NewTestDatabase(t)
	kms, _ := keys.NewInMemory(context.Background())
	env := serverenv.New(ctx, serverenv.WithKeyManager(kms), serverenv.WithDatabase(testDB))

	testCases := []struct {
		name string
	}{
		{
			name: "empty_db",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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
		})
	}
}
