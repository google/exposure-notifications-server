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

	"github.com/google/exposure-notifications-server/internal/mirror/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRenderMirrors(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	mirror := &model.Mirror{}
	m["mirror"] = mirror

	testRenderTemplate(t, "mirror", m)
}

func TestPopulateMirror(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		form *mirrorFormData
		exp  *model.Mirror
	}{
		{
			name: "default",
			form: &mirrorFormData{
				IndexFile:          "index",
				ExportRoot:         "export",
				CloudStorageBucket: "bucket",
				FilenameRoot:       "root",
				FilenameRewrite:    "rewrite",
			},
			exp: &model.Mirror{
				IndexFile:          "index",
				ExportRoot:         "export",
				CloudStorageBucket: "bucket",
				FilenameRoot:       "root",
				FilenameRewrite:    stringPtr("rewrite"),
			},
		},
		{
			name: "no_rewrite",
			form: &mirrorFormData{
				IndexFile:          "index",
				ExportRoot:         "export",
				CloudStorageBucket: "bucket",
				FilenameRoot:       "root",
				FilenameRewrite:    "",
			},
			exp: &model.Mirror{
				IndexFile:          "index",
				ExportRoot:         "export",
				CloudStorageBucket: "bucket",
				FilenameRoot:       "root",
				FilenameRewrite:    nil,
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var m model.Mirror
			tc.form.PopulateMirror(&m)

			opts := cmp.Options{cmpopts.IgnoreUnexported(model.Mirror{})}
			if diff := cmp.Diff(tc.exp, &m, opts); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestHandleMirrorsShow(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()

	mirror := &model.Mirror{
		IndexFile:          "index",
		ExportRoot:         "export",
		CloudStorageBucket: "bucket",
		FilenameRoot:       "root",
		FilenameRewrite:    stringPtr("rewrite"),
	}
	mirrorDB := database.New(db)
	if err := mirrorDB.AddMirror(ctx, mirror); err != nil {
		t.Fatalf("error adding mirror: %v", err)
	}

	cases := []struct {
		name   string
		id     string
		status int
		want   []string
	}{
		{
			name:   "lookup_existing",
			id:     fmt.Sprintf("%d", mirror.ID),
			status: 200,
			want:   []string{mirror.IndexFile, *mirror.FilenameRewrite},
		},
		{
			name:   "show_new",
			id:     "0",
			status: 200,
			want:   []string{"New Authorized Health Authority"},
		},
		{
			name:   "bad_id",
			id:     "banana",
			status: 500,
			want:   []string{"unable to parse `id` param"},
		},
		{
			name:   "non_existing",
			id:     "123",
			status: 500,
			want:   []string{"error loading authorized app"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/:id", s.HandleMirrorsShow())

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

				for _, want := range tc.want {
					if !strings.Contains(string(b), want) {
						t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", b, want)
					}
				}
			}
		})
	}
}

func TestHandleMirrorsSave(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()

	mirror := &model.Mirror{
		IndexFile:          "index",
		ExportRoot:         "export",
		CloudStorageBucket: "bucket",
		FilenameRoot:       "root",
		FilenameRewrite:    stringPtr("rewrite"),
	}
	mirrorDB := database.New(db)
	if err := mirrorDB.AddMirror(ctx, mirror); err != nil {
		t.Fatalf("error adding mirror: %v", err)
	}

	mirrorToDelete := &model.Mirror{
		IndexFile:          "indexD",
		ExportRoot:         "exportD",
		CloudStorageBucket: "bucketD",
		FilenameRoot:       "rootD",
		FilenameRewrite:    stringPtr("rewriteD"),
	}
	if err := mirrorDB.AddMirror(ctx, mirrorToDelete); err != nil {
		t.Fatalf("error adding mirror: %v", err)
	}

	cases := []struct {
		name   string
		id     string
		seed   *model.Mirror
		form   *mirrorFormData
		status int
		want   []string
	}{
		// Create
		{
			name: "create_new",
			id:   "0",
			form: &mirrorFormData{
				Action:             "save",
				IndexFile:          "index1",
				ExportRoot:         "export1",
				CloudStorageBucket: "bucket1",
				FilenameRoot:       "root1",
				FilenameRewrite:    "rewrite1",
			},
			status: 303,
		},
		{
			name: "update_unknown",
			id:   "123",
			form: &mirrorFormData{
				Action:             "save",
				IndexFile:          "index1",
				ExportRoot:         "export1",
				CloudStorageBucket: "bucket1",
				FilenameRoot:       "root1",
				FilenameRewrite:    "rewrite1",
			},
			status: 500,
			want:   []string{"Error loading mirror"},
		},
		{
			name: "update_existing",
			id:   fmt.Sprintf("%d", mirror.ID),
			form: &mirrorFormData{
				Action:             "save",
				IndexFile:          "index2",
				ExportRoot:         "export2",
				CloudStorageBucket: "bucket2",
				FilenameRoot:       "root2",
				FilenameRewrite:    "rewrite2",
			},
			status: 303,
			want:   []string{"Updated mirror"},
		},
		{
			name: "update_bad_id",
			id:   "banana",
			form: &mirrorFormData{
				Action:             "save",
				IndexFile:          "index2",
				ExportRoot:         "export2",
				CloudStorageBucket: "bucket2",
				FilenameRoot:       "root2",
				FilenameRewrite:    "rewrite2",
			},
			status: 500,
			want:   []string{"unable to parse `id` param"},
		},

		// Delete
		{
			name: "delete_unknown",
			id:   "123",
			form: &mirrorFormData{
				Action:             "delete",
				IndexFile:          "index1",
				ExportRoot:         "export1",
				CloudStorageBucket: "bucket1",
				FilenameRoot:       "root1",
				FilenameRewrite:    "rewrite1",
			},
			status: 500,
			want:   []string{"failed to get mirror 123"},
		},
		{
			name: "delete_existing",
			id:   fmt.Sprintf("%d", mirrorToDelete.ID),
			form: &mirrorFormData{
				Action:             "delete",
				IndexFile:          "index1",
				ExportRoot:         "export1",
				CloudStorageBucket: "bucket1",
				FilenameRoot:       "root1",
				FilenameRewrite:    "rewrite1",
			},
			status: 303,
			want:   []string{"Deleted mirror"},
		},
		{
			name: "delete_bad_id",
			id:   "banana",
			form: &mirrorFormData{
				Action:             "delete",
				IndexFile:          "index2",
				ExportRoot:         "export2",
				CloudStorageBucket: "bucket2",
				FilenameRoot:       "root2",
				FilenameRewrite:    "rewrite2",
			},
			status: 500,
			want:   []string{"unable to parse `id` param"},
		},

		// Unknown action
		{
			name: "delete_bad_id",
			id:   "1",
			form: &mirrorFormData{
				Action:             "banana",
				IndexFile:          "index2",
				ExportRoot:         "export2",
				CloudStorageBucket: "bucket2",
				FilenameRoot:       "root2",
				FilenameRewrite:    "rewrite2",
			},
			status: 500,
			want:   []string{"Invalid form action"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := mirrorDB.AddMirror(ctx, tc.seed); err != nil {
					t.Fatalf("failed to add mirror: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id", s.HandleMirrorsSave())

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

				for _, want := range tc.want {
					if !strings.Contains(string(b), want) {
						t.Errorf("expected\n\n%s\n\nto contain\n\n%s\n\n", b, want)
					}
				}
			}
		})
	}
}
