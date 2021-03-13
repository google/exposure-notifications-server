// Copyright 2021 Google LLC
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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
)

func TestRenderSignatureInfo(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	sigInfo := &model.SignatureInfo{}
	m["siginfo"] = sigInfo

	recorder := httptest.NewRecorder()
	config := Config{}
	err := config.RenderTemplate(recorder, "siginfo", m)
	if err != nil {
		t.Fatalf("error rendering template: %v", err)
	}
}

func TestHandleSignatureInfoSave(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	db, s := newTestServer(t)
	exportDB := database.New(db)

	cases := []struct {
		name     string
		seed     *model.SignatureInfo
		form     *signatureInfoFormData
		want     []string
		idChange func(string) string
	}{
		{
			name: "create_new",
			form: &signatureInfoFormData{
				SigningKey:        "/test/case/key/1",
				SigningKeyID:      "foo",
				SigningKeyVersion: "v42",
			},
			want: []string{"/test/case/key/1"},
		},
		{
			name: "bad_id",
			form: &signatureInfoFormData{
				SigningKey:        "/test/case/key/2",
				SigningKeyID:      "foo-2",
				SigningKeyVersion: "v42-2",
			},
			want:     []string{"Unable to parse `id` param"},
			idChange: func(s string) string { return "garbage" },
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := exportDB.AddSignatureInfo(ctx, tc.seed); err != nil {
					t.Fatalf("error adding signature info: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id", s.HandleSignatureInfosSave())

			id := "0"
			if tc.seed != nil {
				id = fmt.Sprintf("%d", tc.seed.ID)
			}
			if tc.idChange != nil {
				id = tc.idChange(id)
			}

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			ctx := context.Background()
			req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s", server.URL, id), strings.NewReader(form.Encode()))
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			client := server.Client()
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}

			mustFindStrings(t, resp, tc.want...)
		})
	}
}

func TestHandleSigntureInfosShow(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	db, s := newTestServer(t)

	info := &model.SignatureInfo{
		SigningKey:        "/path/to/signing/key",
		SigningKeyVersion: "v1",
		SigningKeyID:      "mvv",
	}
	exportDB := database.New(db)
	if err := exportDB.AddSignatureInfo(ctx, info); err != nil {
		t.Fatalf("error adding signature info: %v", err)
	}

	cases := []struct {
		name string
		id   string
		want []string
	}{
		{
			name: "lookup_existing",
			id:   fmt.Sprintf("%d", info.ID),
			want: []string{info.SigningKey, info.SigningKeyVersion, info.SigningKeyID},
		},
		{
			name: "show_new",
			id:   "0",
			want: []string{"New Signature Info"},
		},
		{
			name: "non_existing",
			id:   "42",
			want: []string{"error loading signature info"},
		},
		{
			name: "invalid_id",
			id:   "nan",
			want: []string{"Unable to parse `id` param."},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/:id", s.HandleSignatureInfosShow())

			ctx := context.Background()
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", server.URL, tc.id), nil)
			client := server.Client()

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}

			mustFindStrings(t, resp, tc.want...)
		})
	}
}
