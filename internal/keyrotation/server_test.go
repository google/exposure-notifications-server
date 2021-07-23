// Copyright 2020 the Exposure Notifications Server authors
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

package keyrotation

import (
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/revision"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

func TestNewRotationHandler(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	kms := keys.TestKeyManager(t)

	testCases := []struct {
		name string
		env  *serverenv.ServerEnv
		err  error
	}{
		{
			name: "nil_database",
			env:  serverenv.New(ctx),
			err:  fmt.Errorf("missing database in server environment"),
		},
		{
			name: "nil_key_manager",
			env:  serverenv.New(ctx, serverenv.WithDatabase(testDB)),
			err:  fmt.Errorf("missing key manager in server environment"),
		},
		{
			name: "fully_specified",
			env:  serverenv.New(ctx, serverenv.WithKeyManager(kms), serverenv.WithDatabase(testDB)),
			err:  nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			keyID := keys.TestEncryptionKey(t, kms)

			config := &Config{
				RevisionToken: revision.Config{KeyID: keyID},
			}
			config.RevisionToken.KeyID = keyID

			got, err := NewServer(config, tc.env)
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Fatalf("got %+v: want %v", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			} else if got.env != tc.env {
				t.Fatalf("got %+v: want %v", got.env, tc.env)
			}
		})
	}
}
