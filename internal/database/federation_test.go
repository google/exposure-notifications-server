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
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestFederation(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	ts := time.Date(2020, 5, 6, 0, 0, 0, 0, time.UTC)
	want := &model.FederationQuery{
		QueryID:        "qid",
		ServerAddr:     "addr",
		IncludeRegions: []string{"MX"},
		ExcludeRegions: []string{"CA"},
		LastTimestamp:  ts,
	}
	// GetFederationQuery should fail if not found.
	if _, err := testDB.GetFederationQuery(ctx, want.QueryID); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Add a query, then get it.
	if err := testDB.AddFederationQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := testDB.GetFederationQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// AddFederationQuery should overwrite.
	want.ServerAddr = "addr2"
	if err := testDB.AddFederationQuery(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = testDB.GetFederationQuery(ctx, want.QueryID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// GetFederationSync should fail if not found.
	if _, err := testDB.GetFederationSync(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}

	// Start a sync.
	now := time.Now().Truncate(time.Microsecond)
	syncID, finalize, err := testDB.StartFederationSync(ctx, want, now)
	if err != nil {
		t.Fatal(err)
	}
	gotSync, err := testDB.GetFederationSync(ctx, syncID)
	if err != nil {
		t.Fatal(err)
	}
	wantSync := &model.FederationSync{
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
	gotSync, err = testDB.GetFederationSync(ctx, syncID)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantSync, gotSync, cmpopts.EquateApproxTime(time.Minute)); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestFederationAuthorization(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	want := &model.FederationAuthorization{
		Issuer:         "iss",
		Subject:        "sub",
		Audience:       "aud",
		Note:           "some note",
		IncludeRegions: []string{"MX"},
		ExcludeRegions: []string{"CA"},
	}

	// GetFederationAuthorization should fail if not found.
	if _, err := testDB.GetFederationAuthorization(ctx, want.Issuer, want.Subject); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	// Add a query, then get it.
	if err := testDB.AddFederationAuthorization(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := testDB.GetFederationAuthorization(ctx, want.Issuer, want.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// AddFederationAuthorization should overwrite.
	want.Note = "a different note"
	if err := testDB.AddFederationAuthorization(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = testDB.GetFederationAuthorization(ctx, want.Issuer, want.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
