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

package export

import (
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

// TestNewServer tests NewServer().
func TestNewServer(t *testing.T) {
	t.Parallel()

	emptyStorage := &storage.Memory{}
	emptyKMS := &keys.Filesystem{}
	emptyDB := &database.DB{}
	ctx := project.TestContext(t)

	testCases := []struct {
		name string
		env  *serverenv.ServerEnv
		err  error
	}{
		{
			name: "nil Blobstore",
			env:  serverenv.New(ctx),
			err:  fmt.Errorf("export.NewBatchServer requires Blobstore present in the ServerEnv"),
		},
		{
			name: "nil Database",
			env: serverenv.New(ctx,
				serverenv.WithBlobStorage(emptyStorage),
			),
			err: fmt.Errorf("export.NewBatchServer requires Database present in the ServerEnv"),
		},
		{
			name: "nil KeyManager",
			env: serverenv.New(ctx,
				serverenv.WithBlobStorage(emptyStorage),
				serverenv.WithDatabase(emptyDB),
			),
			err: fmt.Errorf("export.NewBatchServer requires KeyManager present in the ServerEnv"),
		},
		{
			name: "Fully Specified",
			env: serverenv.New(ctx,
				serverenv.WithBlobStorage(emptyStorage),
				serverenv.WithDatabase(emptyDB),
				serverenv.WithKeyManager(emptyKMS),
			),
			err: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewServer(&Config{}, tc.env)
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
