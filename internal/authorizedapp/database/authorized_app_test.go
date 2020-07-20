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
	"crypto/ecdsa"
	"testing"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAuthorizedAppLifecycle(t *testing.T) {
	t.Parallel()

	testDB := database.NewTestDatabase(t)
	ctx := context.Background()
	aadb := New(testDB)

	source := &model.AuthorizedApp{
		AppPackageName:            "myapp",
		AllowedRegions:            map[string]struct{}{"US": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}, 2: {}},
	}

	if err := aadb.InsertAuthorizedApp(ctx, source); err != nil {
		t.Fatal(err)
	}

	readBack, err := aadb.GetAuthorizedApp(ctx, source.AppPackageName)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(source, readBack); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	source.AllowedRegions["CA"] = struct{}{}
	if err := aadb.UpdateAuthorizedApp(ctx, source.AppPackageName, source); err != nil {
		t.Fatal(err)
	}

	readBack, err = aadb.GetAuthorizedApp(ctx, source.AppPackageName)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(source, readBack); diff != "" {
		t.Fatalf("mismatch (-want, +got):\n%s", diff)
	}

	if err := aadb.DeleteAuthorizedApp(ctx, source.AppPackageName); err != nil {
		t.Fatal(err)
	}

	readBack, err = aadb.GetAuthorizedApp(ctx, source.AppPackageName)
	if err != nil {
		t.Errorf("unexpected error seen: %v", err)
	}
	if readBack != nil {
		t.Fatal("expected record to be deleted, but it wasn't")
	}
}

func TestGetAuthorizedApp(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cases := []struct {
		name string
		sql  string
		args []interface{}
		exp  *model.AuthorizedApp
		err  bool
	}{
		{
			name: "bare",
			sql: `
				INSERT INTO AuthorizedApp (app_package_name, allowed_regions, allowed_health_authority_ids)
				VALUES (LOWER($1), $2, $3)
			`,
			args: []interface{}{"myapp", []string{"US"}, []int64{1}},
			exp: &model.AuthorizedApp{
				AppPackageName:            "myapp",
				AllowedRegions:            map[string]struct{}{"US": {}},
				AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}},
			},
		},
		{
			name: "all_regions",
			sql: `
				INSERT INTO AuthorizedApp (app_package_name, allowed_regions, allowed_health_authority_ids)
				VALUES (LOWER($1), $2, $3)
			`,
			args: []interface{}{"myapp", []string{}, []int64{1}},
			exp: &model.AuthorizedApp{
				AppPackageName:            "myapp",
				AllowedRegions:            map[string]struct{}{},
				AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}},
			},
		},
		{
			name: "not_found",
			sql:  "",
			args: nil,
			exp:  nil,
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			testDB := database.NewTestDatabase(t)

			// Acquire a connection
			conn, err := testDB.Pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()

			// Insert the data
			if _, err := conn.Exec(ctx, c.sql, c.args...); err != nil {
				t.Fatal(err)
			}

			config, err := New(testDB).GetAuthorizedApp(ctx, "myapp")
			if (err != nil) != c.err {
				t.Fatal(err)
			}

			// Compare, ignoring the private key part
			opts := cmpopts.IgnoreTypes(new(ecdsa.PrivateKey))
			if diff := cmp.Diff(config, c.exp, opts); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
