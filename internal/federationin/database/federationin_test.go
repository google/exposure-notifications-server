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
	"errors"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/federationin/model"
	"github.com/google/exposure-notifications-server/internal/pb/federation"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/database"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// TestFederationIn tests functions operating over FederationInQuery, FederationInSync.
func TestFederationIn(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	db := New(testDB)

	want := &model.FederationInQuery{
		QueryID:             "qid",
		ServerAddr:          "addr",
		IncludeRegions:      []string{"MX"},
		ExcludeRegions:      []string{"CA"},
		OnlyLocalProvenance: true,
		OnlyTravelers:       false,
	}
	// GetFederationQuery should fail if not found.
	if _, err := db.GetFederationInQuery(ctx, want.QueryID); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Add a query, then get it.
	if err := db.AddFederationInQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetFederationInQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// AddFederationQuery should overwrite.
	want.ServerAddr = "addr2"
	if err := db.AddFederationInQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = db.GetFederationInQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// GetFederationSync should fail if not found.
	if _, err := db.GetFederationInSync(ctx, 1); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Start a sync.
	now := time.Now().Truncate(time.Microsecond)
	syncID, finalize, err := db.StartFederationInSync(ctx, want, now)
	if err != nil {
		t.Fatal(err)
	}
	gotSync, err := db.GetFederationInSync(ctx, syncID)
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
	wantSync.MaxTimestamp = now.Add(time.Hour).UTC()
	wantSync.MaxRevisedTimestamp = now.Add(30 * time.Minute).UTC()
	wantSync.Insertions = 10

	state := federation.FetchState{
		KeyCursor: &federation.Cursor{
			Timestamp: wantSync.MaxTimestamp.Unix(),
		},
		RevisedKeyCursor: &federation.Cursor{
			Timestamp: wantSync.MaxRevisedTimestamp.Unix(),
		},
	}

	// The completed time will be close to the start time.
	wantSync.Completed = now
	if err := finalize(&state, want, wantSync.Insertions); err != nil {
		t.Fatal(err)
	}
	gotSync, err = db.GetFederationInSync(ctx, syncID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantSync, gotSync, cmpopts.EquateApproxTime(time.Minute)); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// Verify query state was updated.
	got, err = db.GetFederationInQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
