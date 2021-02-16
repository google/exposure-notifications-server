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
	"crypto/ecdsa"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestAuthorizedAppInsert_Errors(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	aadb := New(testDB)

	source := &model.AuthorizedApp{}

	validationError := "AuthorizedApp invalid: Health Authority ID cannot be empty"
	if err := aadb.InsertAuthorizedApp(ctx, source); err == nil {
		t.Fatal("expected validation error")
	} else if !strings.Contains(err.Error(), validationError) {
		t.Fatalf("wrong error, want: %q got: %q", validationError, err.Error())
	}

	source.AppPackageName = "foo"
	source.AllowedRegions = map[string]struct{}{"US": {}}
	if err := aadb.InsertAuthorizedApp(ctx, source); err != nil {
		t.Fatal(err)
	}

	if err := aadb.InsertAuthorizedApp(ctx, source); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthorizedAppLifecycle(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
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

	// attempt to delete already deleted app.
	expectedError := "no rows were deleted"
	if err := aadb.DeleteAuthorizedApp(ctx, source.AppPackageName); err == nil {
		t.Fatal("expected error deleting already deleted app")
	} else if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("wrong error, want: %q got: %q", expectedError, err.Error())
	}
}

func TestUpdateAuthorizedApp_NoRows(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	aadb := New(testDB)

	source := &model.AuthorizedApp{
		AppPackageName:            "myapp",
		AllowedRegions:            map[string]struct{}{"US": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}, 2: {}},
	}

	if err := aadb.InsertAuthorizedApp(ctx, source); err != nil {
		t.Fatal(err)
	}

	expectedError := "no rows updated"
	if err := aadb.UpdateAuthorizedApp(ctx, "wrongKey", source); err == nil {
		t.Fatal("expected error, but didn't get one")
	} else if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("wrong error, want: %q got: %q", expectedError, err.Error())
	}
}

func TestUpdateAuthorizedApp_Errors(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	aadb := New(testDB)

	appA := &model.AuthorizedApp{
		AppPackageName:            "appA",
		AllowedRegions:            map[string]struct{}{"US": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}, 2: {}},
	}
	if err := aadb.InsertAuthorizedApp(ctx, appA); err != nil {
		t.Fatal(err)
	}

	appB := &model.AuthorizedApp{
		AppPackageName:            "appB",
		AllowedRegions:            map[string]struct{}{"US": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}, 2: {}},
	}
	if err := aadb.InsertAuthorizedApp(ctx, appB); err != nil {
		t.Fatal(err)
	}

	appB.AppPackageName = appA.AppPackageName

	expectedError := "updating authorizedapp: ERROR: duplicate key value violates unique constraint"
	if err := aadb.UpdateAuthorizedApp(ctx, "appB", appB); err == nil {
		t.Fatal("expected error, but didn't get one")
	} else if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("wrong error, want: %q got: %q", expectedError, err.Error())
	}
}

func TestGetAuthorizedApp(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

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

			testDB, _ := testDatabaseInstance.NewDatabase(t)

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

func TestListAuthorizedApps(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)
	testDB, _ := testDatabaseInstance.NewDatabase(t)
	aadb := New(testDB)

	apps := []*model.AuthorizedApp{
		{
			AppPackageName: "apple",
			AllowedRegions: map[string]struct{}{"US": {}},
			AllowedHealthAuthorityIDs: map[int64]struct{}{
				1: {},
			},
		},
		{
			AppPackageName: "banana",
			AllowedRegions: map[string]struct{}{"US": {}},
			AllowedHealthAuthorityIDs: map[int64]struct{}{
				3: {},
			},
		},
		{
			AppPackageName: "cherry",
			AllowedRegions: map[string]struct{}{"US": {}},
			AllowedHealthAuthorityIDs: map[int64]struct{}{
				2: {},
			},
		},
	}

	for _, newApp := range apps {
		if err := aadb.InsertAuthorizedApp(ctx, newApp); err != nil {
			t.Fatal(err)
		}
	}

	got, err := aadb.ListAuthorizedApps(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(apps, got); diff != "" {
		t.Errorf("mismatch (-want, +got):\n%s", diff)
	}
}
