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

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/federationin/model"

	"github.com/google/go-cmp/cmp"
)

// TestFederationOutAuthorization tests the functions accessing the FederationOutAuthorization table.
func TestFederationOutAuthorization(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	want := &model.FederationOutAuthorization{
		Issuer:         "iss",
		Subject:        "sub",
		Audience:       "aud",
		Note:           "some note",
		IncludeRegions: []string{"MX"},
		ExcludeRegions: []string{"CA"},
	}

	// GetFederationOutAuthorization should fail if not found.
	if _, err := New(testDB).GetFederationOutAuthorization(ctx, want.Issuer, want.Subject); !errors.Is(err, database.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	// Add a query, then get it.
	if err := New(testDB).AddFederationOutAuthorization(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := New(testDB).GetFederationOutAuthorization(ctx, want.Issuer, want.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}

	// AddFederationOutAuthorization should overwrite.
	want.Note = "a different note"
	if err := New(testDB).AddFederationOutAuthorization(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = New(testDB).GetFederationOutAuthorization(ctx, want.Issuer, want.Subject)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
