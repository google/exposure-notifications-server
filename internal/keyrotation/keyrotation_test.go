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

package keyrotation

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

func TestNewRotationHandler(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB := database.NewTestDatabase(t)
	emptyKMS := &keys.GoogleCloudKMS{}

	testCases := []struct {
		name string
		env  *serverenv.ServerEnv
		err  error
	}{
		{
			name: "nil Database",
			env:  serverenv.New(ctx),
			err:  fmt.Errorf("missing database in server environment"),
		},
		{
			name: "nil Key manager",
			env:  serverenv.New(ctx, serverenv.WithDatabase(testDB)),
			err:  fmt.Errorf("missing key manager in server environment"),
		},
		{
			name: "Fully Specified",
			env:  serverenv.New(ctx, serverenv.WithKeyManager(emptyKMS), serverenv.WithDatabase(testDB)),
			err:  nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewRotationHandler(&Config{}, tc.env)
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Fatalf("got %+v: want %v", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			} else {
				handler, ok := got.(*handler)
				if !ok {
					t.Fatal("handler does not satisfy http.Handler interface")
				} else if handler.env != tc.env {
					t.Fatalf("got %+v: want %v", handler.env, tc.env)
				}
			}
		})
	}
}
