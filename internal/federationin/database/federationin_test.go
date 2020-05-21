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

package database

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	coredb "github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/federationin/model"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var testDB *coredb.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	if os.Getenv("DB_USER") != "" {
		var err error
		testDB, err = coredb.CreateTestDB(ctx, "federationin")
		if err != nil {
			log.Fatalf("creating test DB: %v", err)
		}
	}
	os.Exit(m.Run())
}

// TestFederationIn tests functions operating over FederationInQuery, FederationInSync.
func TestFederationIn(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer coredb.ResetTestDB(t, testDB)
	ctx := context.Background()

	ts := time.Date(2020, 5, 6, 0, 0, 0, 0, time.UTC)
	want := &model.FederationInQuery{
		QueryID:        "qid",
		ServerAddr:     "addr",
		IncludeRegions: []string{"MX"},
		ExcludeRegions: []string{"CA"},
		LastTimestamp:  ts,
	}
	// GetFederationQuery should fail if not found.
	if _, err := New(testDB).GetFederationInQuery(ctx, want.QueryID); !errors.Is(err, coredb.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Add a query, then get it.
	if err := New(testDB).AddFederationInQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := New(testDB).GetFederationInQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// AddFederationQuery should overwrite.
	want.ServerAddr = "addr2"
	if err := New(testDB).AddFederationInQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = New(testDB).GetFederationInQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// GetFederationSync should fail if not found.
	if _, err := New(testDB).GetFederationInSync(ctx, 1); !errors.Is(err, coredb.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Start a sync.
	now := time.Now().Truncate(time.Microsecond)
	syncID, finalize, err := New(testDB).StartFederationInSync(ctx, want, now)
	if err != nil {
		t.Fatal(err)
	}
	gotSync, err := New(testDB).GetFederationInSync(ctx, syncID)
	if err != nil {
		t.Fatal(err)
	}
	wantSync := &model.FederationInSync{
		SyncID:  syncID,
		QueryID: want.QueryID,
		Started: now,
	}
	if diff := cmp.Diff(wantSync, gotSync); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Finalize the sync.
	wantSync.MaxTimestamp = now.Add(time.Hour)
	wantSync.Insertions = 10

	// The completed time will be close to the start time.
	wantSync.Completed = now
	if err := finalize(wantSync.MaxTimestamp, wantSync.Insertions); err != nil {
		t.Fatal(err)
	}
	gotSync, err = New(testDB).GetFederationInSync(ctx, syncID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantSync, gotSync, cmpopts.EquateApproxTime(time.Minute)); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
