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
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/verification/model"
	"google.golang.org/protobuf/proto"

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

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	haDB := New(testDB)

	_, err := haDB.GetHealthAuthority(ctx, "does-not-exist")
	if err == nil {
		t.Fatal("missing error")
	}
	if !errors.Is(err, ErrHealthAuthorityNotFound) {
		t.Fatalf("wrong error want: %v got: %v", ErrHealthAuthorityNotFound, err)
	}
}

func TestAddHealthAuthorityErrors(t *testing.T) {
	t.Parallel()

	testDB, _ := testDatabaseInstance.NewDatabase(t)
	haDB := New(testDB)

	cases := []struct {
		name string
		ha   *model.HealthAuthority
		want string
	}{
		{
			name: "nil",
			ha:   nil,
			want: "provided HealthAuthority cannot be nil",
		},
		{
			name: "with_keys",
			ha: &model.HealthAuthority{
				Keys: []*model.HealthAuthorityKey{
					{
						AuthorityID: 0,
					},
				},
			},
			want: "unable to insert health authority with keys, attach keys later",
		},
		{
			name: "invalid",
			ha:   &model.HealthAuthority{},
			want: "issuer cannot be empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if err := haDB.AddHealthAuthority(ctx, tc.ha); err == nil {
				t.Error("missing expected error")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("wrong error: want: %q got: %q", tc.want, err.Error())
			}
		})
	}
}

func TestAddRetrieveHealthAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)

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

	want.EnableStatsAPI = true
	if err := haDB.UpdateHealthAuthority(ctx, want); err != nil {
		t.Fatal(err)
	}
}

func TestAddRetrieveHealthAuthorityKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)

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

func TestListAllHealthAuthoritiesWithoutKeys(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	testDB, _ := testDatabaseInstance.NewDatabase(t)

	want := []*model.HealthAuthority{
		{
			Issuer:         "doh.mystate.gov",
			Audience:       "ens.usacovid.org",
			Name:           "My State Department of Healthiness",
			EnableStatsAPI: true,
			JwksURI:        proto.String("https://www.example.com/.auth/keys.json"),
		},
		{
			Issuer:         "other.doh.mystate.gov",
			Audience:       "other.ens.usacovid.org",
			Name:           "other.My State Department of Healthiness",
			EnableStatsAPI: true,
			JwksURI:        proto.String("https://www.example.com/.auth/keys2.json"),
		},
	}

	haDB := New(testDB)
	for _, ha := range want {
		if err := haDB.AddHealthAuthority(ctx, ha); err != nil {
			t.Fatal(err)
		}
	}

	got, err := haDB.ListAllHealthAuthoritiesWithoutKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}
}
