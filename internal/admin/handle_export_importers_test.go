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
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRenderExportImporters(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	model := new(model.ExportImport)
	m["model"] = model

	testRenderTemplate(t, "export-importer", m)
}

func TestBuildExportImporterModel(t *testing.T) {
	t.Parallel()

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	cases := []struct {
		name string
		form *exportImporterFormData
		exp  *model.ExportImport
		err  string
	}{
		{
			name: "default",
			form: &exportImporterFormData{
				IndexFile:  "index.txt",
				ExportRoot: "root",
				Region:     "TEST",
				Travelers:  true,
				FromDate:   "2021-01-02",
				FromTime:   "09:23",
				ThruDate:   "2022-01-02",
				ThruTime:   "10:34",
			},
			exp: &model.ExportImport{
				IndexFile:  "index.txt",
				ExportRoot: "root",
				Region:     "TEST",
				Traveler:   true,
				From:       from,
				Thru:       &thru,
			},
		},
		{
			name: "bad_from",
			form: &exportImporterFormData{
				FromDate: "banana",
				FromTime: "apple",
			},
			err: "invalid from time",
		},
		{
			name: "zero_time",
			form: &exportImporterFormData{
				FromDate: "",
				FromTime: "",
			},
			exp: &model.ExportImport{
				From: time.Now().UTC().Add(1 * time.Minute),
			},
		},
		{
			name: "bad_thru",
			form: &exportImporterFormData{
				ThruDate: "banana",
				ThruTime: "apple",
			},
			err: "invalid thru time",
		},
		{
			name: "zero_thru",
			form: &exportImporterFormData{
				ThruDate: "",
				ThruTime: "",
			},
			exp: &model.ExportImport{
				From: time.Now().UTC().Add(1 * time.Minute),
				Thru: nil,
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var key model.ExportImport
			err := tc.form.BuildExportImporterModel(&key)
			if err != nil {
				if tc.err == "" {
					t.Fatal(err)
				}
				if got, want := err.Error(), tc.err; !strings.Contains(got, want) {
					t.Errorf("expected %q to contain %q", got, want)
				}
			}

			if tc.err == "" {
				opts := cmp.Options{cmpopts.EquateApproxTime(5 * time.Minute)}
				if diff := cmp.Diff(tc.exp, &key, opts); diff != "" {
					t.Errorf("mismatch (-want, +got):\n%s", diff)
				}
			}
		})
	}
}

func TestHandleExportImportersShow(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	exportimportDB := database.New(db)

	exportImport := &model.ExportImport{
		IndexFile:  "index.txt",
		ExportRoot: "root",
		Region:     "TEST",
		Traveler:   true,
		From:       time.Now().UTC().Add(-24 * time.Hour),
	}
	if err := exportimportDB.AddConfig(ctx, exportImport); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		id     string
		status int
		want   []string
	}{
		{
			name:   "new",
			id:     "0",
			status: 200,
			want:   []string{"New Export Importer Config"},
		},
		{
			name:   "existing",
			id:     fmt.Sprintf("%d", exportImport.ID),
			status: 200,
			want:   []string{"Edit Export Importer Config", exportImport.ExportRoot},
		},
		{
			name:   "non_existing",
			id:     "123456",
			status: 500,
			want:   []string{"failed to lookup export importer config"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/:id", s.HandleExportImportersShow())

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", server.URL, tc.id), nil)
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

			mustFindStrings(t, resp, tc.want...)
		})
	}
}

func TestHandleExportImportersSave(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	exportimportDB := database.New(db)

	exportImport := &model.ExportImport{
		IndexFile:  "index.txt",
		ExportRoot: "root",
		Region:     "TEST",
		Traveler:   true,
		From:       time.Now().UTC().Add(-24 * time.Hour),
	}
	if err := exportimportDB.AddConfig(ctx, exportImport); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		id     string
		seed   *model.ExportImport
		form   *exportImporterFormData
		status int
		want   []string
	}{
		// create
		{
			name: "create_new",
			id:   "0",
			form: &exportImporterFormData{
				IndexFile:  "index.txt",
				ExportRoot: "root",
				Region:     "TEST",
				Travelers:  true,
				FromDate:   "2021-01-02",
				FromTime:   "09:23",
				ThruDate:   "2022-01-02",
				ThruTime:   "10:34",
			},
			status: 303,
		},

		// update
		{
			name: "update_unknown",
			id:   "123456",
			form: &exportImporterFormData{
				IndexFile: "index.txt",
			},
			status: 500,
			want:   []string{"failed to lookup export importer config"},
		},
		{
			name: "update_bad_id",
			id:   "banana",
			form: &exportImporterFormData{
				IndexFile: "index.txt",
			},
			status: 500,
			want:   []string{"failed to parse"},
		},
		{
			name: "update_request",
			id:   fmt.Sprintf("%d", exportImport.ID),
			form: &exportImporterFormData{
				IndexFile: "index.txt",
				FromDate:  "nope",
			},
			status: 500,
			want:   []string{"failed to build export importer config"},
		},
		{
			name: "update_existing",
			id:   fmt.Sprintf("%d", exportImport.ID),
			form: &exportImporterFormData{
				IndexFile: "index.txt",
			},
			status: 303,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := exportimportDB.AddConfig(ctx, tc.seed); err != nil {
					t.Fatalf("failed to add export import config: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id", s.HandleExportImportersSave())

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s", server.URL, tc.id), strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
			client := server.Client()
			client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}

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
