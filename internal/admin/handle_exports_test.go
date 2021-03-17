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
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRenderExports(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	exportConfig := &model.ExportConfig{}
	m["export"] = exportConfig

	sigInfos := []*model.SignatureInfo{
		{ID: 5},
	}
	usedSigInfos := map[int64]bool{5: true}
	m["usedSigInfos"] = usedSigInfos
	m["siginfos"] = sigInfos

	testRenderTemplate(t, "export", m)
}

func TestSplitRegions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		r string
		e []string
	}{
		{"", []string{}},
		{"  test  ", []string{"test"}},
		{"test\n\rfoo", []string{"foo", "test"}},
		{"test\n\rfoo bar\n\r", []string{"foo bar", "test"}},
		{"test\n\rfoo bar\n\r  ", []string{"foo bar", "test"}},
		{"test\nfoo\n", []string{"foo", "test"}},
	}

	for i, test := range tests {
		if res := splitRegions(test.r); !reflect.DeepEqual(res, test.e) {
			t.Errorf("[%d] splitRegions(%v) = %v, expected %v", i, test.r, res, test.e)
		}
	}
}

func TestPopulateExportConfig(t *testing.T) {
	t.Parallel()

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	cases := []struct {
		name string
		form *exportFormData
		exp  *model.ExportConfig
		err  string
	}{
		{
			name: "default",
			form: &exportFormData{
				OutputRegion:       "TEST",
				InputRegions:       "TEST",
				ExcludeRegions:     "NOT-TEST",
				IncludeTravelers:   true,
				BucketName:         "bucket",
				FilenameRoot:       "root",
				Period:             4 * time.Hour,
				FromDate:           "2021-01-02",
				FromTime:           "09:23",
				ThruDate:           "2022-01-02",
				ThruTime:           "10:34",
				MaxRecordsOverride: 10,
			},
			exp: &model.ExportConfig{
				BucketName:         "bucket",
				FilenameRoot:       "root",
				Period:             4 * time.Hour,
				OutputRegion:       "TEST",
				InputRegions:       []string{"TEST"},
				ExcludeRegions:     []string{"NOT-TEST"},
				IncludeTravelers:   true,
				From:               from,
				Thru:               thru,
				SignatureInfoIDs:   nil,
				MaxRecordsOverride: intPtr(10),
			},
		},
		{
			name: "bad_from",
			form: &exportFormData{
				FromDate: "banana",
				FromTime: "apple",
			},
			err: "invalid from time",
		},
		{
			name: "bad_thru",
			form: &exportFormData{
				ThruDate: "banana",
				ThruTime: "apple",
			},
			err: "invalid thru time",
		},
		{
			name: "travelers_conflict",
			form: &exportFormData{
				IncludeTravelers: true,
				OnlyNonTravelers: true,
			},
			err: "cannot have both 'include travelers', and 'only non-travelers'",
		},
		{
			name: "too_many_ids",
			form: &exportFormData{
				SigInfoIDs: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			},
			err: "too many signing keys selected",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var key model.ExportConfig
			err := tc.form.PopulateExportConfig(&key)
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

func TestHandleExportsShow(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	env, s := newTestServer(t)
	db := env.Database()
	exportDB := database.New(db)

	exportConfig := &model.ExportConfig{
		BucketName:         "bucket",
		FilenameRoot:       "root",
		Period:             4 * time.Hour,
		OutputRegion:       "TEST",
		InputRegions:       []string{"TEST"},
		ExcludeRegions:     []string{"NOT-TEST"},
		IncludeTravelers:   true,
		From:               from,
		Thru:               thru,
		SignatureInfoIDs:   []int64{1},
		MaxRecordsOverride: intPtr(10),
	}
	if err := exportDB.AddExportConfig(ctx, exportConfig); err != nil {
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
			want:   []string{"New Export Config"},
		},
		{
			name:   "existing",
			id:     fmt.Sprintf("%d", exportConfig.ConfigID),
			status: 200,
			want:   []string{"Edit Export Config", exportConfig.BucketName},
		},
		{
			name:   "bad_id",
			id:     "banana",
			status: 500,
			want:   []string{"failed to parse"},
		},
		{
			name:   "non_existing",
			id:     "123456",
			status: 500,
			want:   []string{"Failed to load export config"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/:id", s.HandleExportsShow())

			ctx := context.Background()
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

func TestHandleExportsSave(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	env, s := newTestServer(t)
	db := env.Database()
	exportDB := database.New(db)

	exportConfig := &model.ExportConfig{
		BucketName:         "bucket",
		FilenameRoot:       "root",
		Period:             4 * time.Hour,
		OutputRegion:       "TEST",
		InputRegions:       []string{"TEST"},
		ExcludeRegions:     []string{"NOT-TEST"},
		IncludeTravelers:   true,
		From:               from,
		Thru:               thru,
		SignatureInfoIDs:   []int64{1},
		MaxRecordsOverride: intPtr(10),
	}
	if err := exportDB.AddExportConfig(ctx, exportConfig); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		id     string
		seed   *model.ExportConfig
		form   *exportFormData
		status int
		want   []string
	}{
		// create
		{
			name: "create_new",
			id:   "0",
			form: &exportFormData{
				OutputRegion:       "TEST",
				InputRegions:       "TEST",
				ExcludeRegions:     "NOT-TEST",
				IncludeTravelers:   true,
				BucketName:         "bucket1",
				FilenameRoot:       "root1",
				Period:             4 * time.Hour,
				FromDate:           "2021-01-02",
				FromTime:           "09:23",
				ThruDate:           "2022-01-02",
				ThruTime:           "10:34",
				MaxRecordsOverride: 10,
			},
			status: 303,
		},

		// update
		{
			name:   "update_unknown",
			id:     "123456",
			form:   &exportFormData{},
			status: 500,
			want:   []string{"Failed to load export config"},
		},
		{
			name:   "update_bad_id",
			id:     "banana",
			form:   &exportFormData{},
			status: 500,
			want:   []string{"failed to parse"},
		},
		{
			name: "update_request",
			id:   fmt.Sprintf("%d", exportConfig.ConfigID),
			form: &exportFormData{
				IncludeTravelers: true,
				OnlyNonTravelers: true,
			},
			status: 500,
			want:   []string{"error processing export config"},
		},
		{
			name: "update_existing",
			id:   fmt.Sprintf("%d", exportConfig.ConfigID),
			form: &exportFormData{
				IncludeTravelers: true,
				Period:           4 * time.Hour,
			},
			status: 303,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := exportDB.AddExportConfig(ctx, tc.seed); err != nil {
					t.Fatalf("failed to add export config: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id", s.HandleExportsSave())

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			ctx := context.Background()
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
