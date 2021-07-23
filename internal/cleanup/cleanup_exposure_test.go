// Copyright 2021 the Exposure Notifications Server authors
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

package cleanup

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

func TestNewExposureServer(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)

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
			name: "Fully Specified",
			env:  serverenv.New(ctx, serverenv.WithDatabase(testDB)),
			err:  nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewExposureServer(&Config{}, tc.env)
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Fatalf("got '%+v': want '%v'", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			} else {
				if got.env != tc.env {
					t.Fatalf("got %+v: want %v", got.env, tc.env)
				}
			}
		})
	}
}

func TestExposureHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	env := serverenv.New(ctx, serverenv.WithDatabase(testDB))

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()

	server, err := NewExposureServer(&Config{
		Timeout: 5 * time.Second,
		TTL:     336 * time.Hour,
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.Routes(ctx)
	handler.ServeHTTP(w, r)

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("expected %d to be %d: %s", got, want, w.Body.String())
	}
}
