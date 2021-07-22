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
	"io"
	"net/http"
	"testing"

	authorizedappmodel "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	exportmodel "github.com/google/exposure-notifications-server/internal/export/model"
	exportimportmodel "github.com/google/exposure-notifications-server/internal/exportimport/model"
	mirrormodel "github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/project"
	verificationmodel "github.com/google/exposure-notifications-server/internal/verification/model"
)

func TestRenderIndex(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	m["apps"] = []*authorizedappmodel.AuthorizedApp{}
	m["healthauthorities"] = []*verificationmodel.HealthAuthority{}
	m["exports"] = []*exportmodel.ExportConfig{}
	m["exportImporters"] = []*exportimportmodel.ExportImport{}
	m["siginfos"] = []*exportmodel.SignatureInfo{}
	m["mirrors"] = []*mirrormodel.Mirror{}

	testRenderTemplate(t, "index", m)
}

func TestHandleIndex(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	_ = db

	cases := []struct {
		name   string
		status int
		want   []string
	}{
		{
			name:   "default",
			status: 200,
			want:   []string{"Admin Console"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/", s.HandleIndex())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			client := server.Client()

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}
			defer resp.Body.Close()

			if got, want := resp.StatusCode, tc.status; got != want {
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatal(err)
				}
				t.Errorf("expected status %d to be %d; headers: %#v; body: %s", got, want, resp.Header, b)
			}

			if len(tc.want) > 0 {
				mustFindStrings(t, resp, tc.want...)
			}
		})
	}
}
