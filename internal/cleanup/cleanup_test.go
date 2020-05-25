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

package cleanup

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
)

func TestNewExposureHandler(t *testing.T) {
	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

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
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewExposureHandler(&Config{}, tc.env)
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Fatalf("got '%+v': want '%v'", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			} else {
				handler, ok := got.(*exposureCleanupHandler)
				if !ok {
					t.Fatalf("exposureCleanupHandler does not satisfy http.Handler interface")
				} else if handler.env != tc.env {
					t.Fatalf("got %+v: want %v", handler.env, tc.env)
				}
			}
		})
	}
}

func TestNewExportHandler(t *testing.T) {
	ctx := context.Background()
	testDB := database.NewTestDatabase(t)
	noopBlobstore, _ := storage.NewNoopBlobstore(ctx)

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
			name: "nil Blobstore",
			env:  serverenv.New(ctx, serverenv.WithDatabase(testDB)),
			err:  fmt.Errorf("missing blobstore in server environment"),
		},
		{
			name: "Fully Specified",
			env:  serverenv.New(ctx, serverenv.WithBlobStorage(noopBlobstore), serverenv.WithDatabase(testDB)),
			err:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewExportHandler(&Config{}, tc.env)
			if tc.err != nil {
				if err.Error() != tc.err.Error() {
					t.Fatalf("got %+v: want %v", err, tc.err)
				}
			} else if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			} else {
				handler, ok := got.(*exportCleanupHandler)
				if !ok {
					t.Fatal("exportCleanupHandler does not satisfy http.Handler interface")
				} else if handler.env != tc.env {
					t.Fatalf("got %+v: want %v", handler.env, tc.env)
				}
			}
		})
	}
}

func TestCutoffDate(t *testing.T) {
	now := time.Now()
	for _, test := range []struct {
		d       time.Duration
		wantDur time.Duration // if zero, then expect an error
	}{
		{216 * time.Hour, 0},                       // 9 days: duration too short
		{-10 * time.Minute, 0},                     // negative
		{241 * time.Hour, (10*24 + 1) * time.Hour}, // 10 days, 1 hour: OK
	} {
		got, err := cutoffDate(test.d)
		if test.wantDur == 0 {
			if err == nil {
				t.Errorf("%q: got no error, wanted one", test.d)
			}
		} else if err != nil {
			t.Errorf("%q: got error %v", test.d, err)
		} else {
			want := now.Add(-test.wantDur)
			diff := got.Sub(want)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Second {
				t.Errorf("%q: got %s, want %s", test.d, got, want)
			}
		}
	}
}
