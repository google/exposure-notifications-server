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

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"

	"github.com/google/go-cmp/cmp"
)

const (
	validPEM = `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEA+k9YktDK3UpOhBIy+O17biuwd/g
IBSEEHOdgpAynz0yrHpkWL6vxjNHxRdWcImZxPgL0NVHMdY4TlsL7qaxBQ==
-----END PUBLIC KEY-----`
)

func TestMissingHealthAuthority(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	haDB := New(testDB)
	ctx := context.Background()

	_, err := haDB.GetHealthAuthority(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("missing error")
	}
	if !errors.Is(err, ErrHealthAuthorityNotFound) {
		t.Fatalf("wrong error want: %v got: %v", ErrHealthAuthorityNotFound, err)
	}

}

func TestAddRetrieveHealthAuthority(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	want := &model.HealthAuthority{
		Issuer:   "doh.mystate.gov",
		Audience: "ens.usacovid.org",
		Name:     "My State Department of Healthiness",
		JwksURI:  nil,
	}

	haDB := New(testDB)
	if err := haDB.AddHealthAuthority(ctx, want); err != nil {
		t.Fatal(err)
	}

	got, err := haDB.GetHealthAuthority(ctx, want.Issuer)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	got, err = haDB.GetHealthAuthorityByID(ctx, want.ID)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}

func TestAddRetrieveHealthAuthorityKeys(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()

	want := &model.HealthAuthority{
		Issuer:   "doh.mystate.gov",
		Audience: "ens.usacovid.org",
		Name:     "My State Department of Healthiness",
	}
	want.SetJWKS("https://www.example.com/.auth/keys.json")

	haDB := New(testDB)
	if err := haDB.AddHealthAuthority(ctx, want); err != nil {
		t.Fatal(err)
	}

	wantKeys := []*model.HealthAuthorityKey{
		{
			Version:      "v1",
			From:         time.Now().Truncate(time.Second),
			PublicKeyPEM: validPEM,
		},
	}

	if err := haDB.AddHealthAuthorityKey(ctx, want, wantKeys[0]); err != nil {
		t.Fatal(err)
	}
	want.Keys = wantKeys

	// Reading back the HA will also read back the keys.
	got, err := haDB.GetHealthAuthority(ctx, want.Issuer)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
