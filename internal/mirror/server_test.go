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

package mirror

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
)

func TestNewServer(t *testing.T) {
	t.Parallel()

	emptyDB := &database.DB{}
	emptyBlobstore := &storage.Noop{}
	ctx := project.TestContext(t)

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
			name: "nil_blobstore",
			env:  serverenv.New(ctx, serverenv.WithDatabase(emptyDB)),
			err:  fmt.Errorf("missing blobstore in server environment"),
		},
		{
			name: "fully_specified",
			env: serverenv.New(ctx,
				serverenv.WithDatabase(emptyDB),
				serverenv.WithBlobStorage(emptyBlobstore)),
			err: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewServer(&Config{}, tc.env)
			if tc.err != nil {
				if got, want := err.Error(), tc.err.Error(); !strings.Contains(got, want) {
					t.Fatalf("expected %q to be %q", got, want)
				}
			} else if err != nil {
				t.Fatal(err)
			} else {
				if got, want := got.env, tc.env; got != want {
					t.Fatalf("expected %#v to be %#v", got, want)
				}
			}
		})
	}
}
