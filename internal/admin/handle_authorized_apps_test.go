// Copyright 2021 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/project"
)

func TestRenderAuthorizedApps(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	authorizedApp := model.NewAuthorizedApp()
	m["app"] = authorizedApp

	testRenderTemplate(t, "authorizedapp", m)
}

func TestHandleAuthorizedAppsShow(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()

	authorizedApp := &model.AuthorizedApp{
		AppPackageName:            "foo.bar.app3",
		AllowedRegions:            map[string]struct{}{"TEST": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}},
	}
	authorizedappDB := database.New(db)
	if err := authorizedappDB.InsertAuthorizedApp(ctx, authorizedApp); err != nil {
		t.Fatalf("error adding signature info: %v", err)
	}

	cases := []struct {
		name string
		apn  string
		want []string
	}{
		{
			name: "lookup_existing",
			apn:  authorizedApp.AppPackageName,
			want: []string{authorizedApp.AppPackageName},
		},
		{
			name: "show_new",
			apn:  "",
			want: []string{"New Authorized Health Authority"},
		},
		{
			name: "non_existing",
			apn:  "banana.apple.com",
			want: []string{"error loading authorized app"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/", s.HandleAuthorizedAppsShow())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/?apn=%s", server.URL, tc.apn), nil)
			if err != nil {
				t.Fatal(err)
			}
			client := server.Client()

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}
			defer resp.Body.Close()

			mustFindStrings(t, resp, tc.want...)
		})
	}
}

func TestHandleAuthorizedAppsSave(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	authorizedappDB := database.New(db)

	authorizedApp := &model.AuthorizedApp{
		AppPackageName:            "foo.bar.app",
		AllowedRegions:            map[string]struct{}{"TEST": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}},
	}
	if err := authorizedappDB.InsertAuthorizedApp(ctx, authorizedApp); err != nil {
		t.Fatalf("error adding signature info: %v", err)
	}

	authorizedAppToDelete := &model.AuthorizedApp{
		AppPackageName:            "foo.bar.delete",
		AllowedRegions:            map[string]struct{}{"TEST": {}},
		AllowedHealthAuthorityIDs: map[int64]struct{}{1: {}},
	}
	if err := authorizedappDB.InsertAuthorizedApp(ctx, authorizedAppToDelete); err != nil {
		t.Fatalf("error adding signature info: %v", err)
	}

	cases := []struct {
		name string
		seed *model.AuthorizedApp
		form *authorizedAppFormData
		want []string
	}{
		// Create
		{
			name: "create_new",
			form: &authorizedAppFormData{
				Action:         "save",
				FormKey:        "",
				AppPackageName: "foo.bar.app1",
				AllowedRegions: "TEST",
			},
			want: []string{"foo.bar.app1"},
		},
		{
			name: "update_unknown",
			form: &authorizedAppFormData{
				Action:         "save",
				FormKey:        base64.StdEncoding.EncodeToString([]byte("not-a-real-app")),
				AppPackageName: "foo.bar.app2",
				AllowedRegions: "TEST",
			},
			want: []string{"Unknown authorized app"},
		},
		{
			name: "update_bad_validation",
			form: &authorizedAppFormData{
				Action:         "save",
				FormKey:        base64.StdEncoding.EncodeToString([]byte(authorizedApp.AppPackageName)),
				AppPackageName: "",
				AllowedRegions: "",
			},
			want: []string{"Health Authority ID cannot be empty", "Regions list cannot be empty"},
		},
		{
			name: "update_existing",
			form: &authorizedAppFormData{
				Action:         "save",
				FormKey:        base64.StdEncoding.EncodeToString([]byte(authorizedApp.AppPackageName)),
				AppPackageName: authorizedApp.AppPackageName,
				AllowedRegions: "TEST",
			},
			want: []string{"Updated authorized app"},
		},

		// Delete
		{
			name: "delete_unknown",
			form: &authorizedAppFormData{
				Action:         "delete",
				FormKey:        base64.StdEncoding.EncodeToString([]byte("not-a-real-app")),
				AppPackageName: "foo.bar.app2",
				AllowedRegions: "TEST",
			},
			want: []string{"no rows were deleted"},
		},
		{
			name: "delete_existing",
			form: &authorizedAppFormData{
				Action:  "delete",
				FormKey: base64.StdEncoding.EncodeToString([]byte(authorizedAppToDelete.AppPackageName)),
			},
			want: []string{"Successfully deleted app"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := authorizedappDB.InsertAuthorizedApp(ctx, tc.seed); err != nil {
					t.Fatalf("failed to add authorized app: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/", s.HandleAuthorizedAppsSave())

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			client := server.Client()
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}
			defer resp.Body.Close()

			mustFindStrings(t, resp, tc.want...)
		})
	}
}
