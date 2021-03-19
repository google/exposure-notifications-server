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

func TestPopulateImportFilePublicKey(t *testing.T) {
	t.Parallel()

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	cases := []struct {
		name string
		form *exportImportKeyForm
		exp  *model.ImportFilePublicKey
		err  string
	}{
		{
			name: "default",
			form: &exportImportKeyForm{
				KeyID:     "key_id",
				Version:   "key_version",
				PublicKey: "PEMPEMPEM",
				FromDate:  "2021-01-02",
				FromTime:  "09:23",
				ThruDate:  "2022-01-02",
				ThruTime:  "10:34",
			},
			exp: &model.ImportFilePublicKey{
				ExportImportID: 123,
				KeyID:          "key_id",
				KeyVersion:     "key_version",
				PublicKeyPEM:   "PEMPEMPEM",
				From:           from,
				Thru:           &thru,
			},
		},
		{
			name: "bad_from",
			form: &exportImportKeyForm{
				FromDate: "banana",
				FromTime: "apple",
			},
			err: "invalid from time",
		},
		{
			name: "zero_time",
			form: &exportImportKeyForm{
				FromDate: "",
				FromTime: "",
			},
			exp: &model.ImportFilePublicKey{
				ExportImportID: 123,
				From:           time.Now().UTC().Add(1 * time.Minute),
			},
		},
		{
			name: "bad_thru",
			form: &exportImportKeyForm{
				ThruDate: "banana",
				ThruTime: "apple",
			},
			err: "invalid thru time",
		},
		{
			name: "zero_thru",
			form: &exportImportKeyForm{
				ThruDate: "",
				ThruTime: "",
			},
			exp: &model.ImportFilePublicKey{
				ExportImportID: 123,
				From:           time.Now().UTC().Add(1 * time.Minute),
				Thru:           nil,
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var key model.ImportFilePublicKey
			err := tc.form.PopulateImportFilePublicKey(123, &key)
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

func TestHandleExportImportKeys(t *testing.T) {
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

	importFilePublicKey := &model.ImportFilePublicKey{
		ExportImportID: exportImport.ID,
		KeyID:          "key_id",
		KeyVersion:     "key_version",
		PublicKeyPEM:   "PEMPEMPEM",
		From:           time.Now().UTC().Add(-5 * time.Minute),
	}
	if err := exportimportDB.AddImportFilePublicKey(ctx, importFilePublicKey); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		action string
		id     string
		keyID  string
		seed   *model.ExportImport
		form   *exportImportKeyForm
		status int
		want   []string
	}{
		{
			name:   "invalid_id",
			action: "create",
			id:     "banana",
			keyID:  "0",
			status: 500,
			want:   []string{"Unable to parse `id` param"},
		},
		{
			name:   "missing_config",
			action: "create",
			id:     "123",
			keyID:  "0",
			status: 500,
			want:   []string{"failed to lookup export importer config"},
		},

		// create
		{
			name:   "create_new",
			action: "create",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  "0",
			form: &exportImportKeyForm{
				KeyID:     "key_id1",
				Version:   "key_version1",
				PublicKey: "PEMPEMPEM1",
				FromDate:  "2021-01-02",
				FromTime:  "09:23",
				ThruDate:  "2022-01-02",
				ThruTime:  "10:34",
			},
			status: 303,
		},
		{
			name:   "create_validation",
			action: "create",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  "0",
			form: &exportImportKeyForm{
				KeyID:     "key_id",
				Version:   "key_version",
				PublicKey: "PEMPEMPEM",
				FromDate:  "bad-date",
				FromTime:  "bad-time",
				ThruDate:  "bad-date",
				ThruTime:  "bad-time",
			},
			status: 500,
			want:   []string{"invalid from time"},
		},

		// revoke
		{
			name:   "revoke",
			action: "revoke",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  importFilePublicKey.KeyID,
			status: 303,
		},
		{
			name:   "revoke_not_exist",
			action: "revoke",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  "not-a-real-key-to-find",
			status: 500,
			want:   []string{"Invalid key specified"},
		},

		// reinstate
		{
			name:   "reinstate",
			action: "reinstate",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  importFilePublicKey.KeyID,
			status: 303,
		},

		// activate
		{
			name:   "activate",
			action: "activate",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  importFilePublicKey.KeyID,
			status: 303,
		},

		// unknown
		{
			name:   "invalid_action",
			action: "nope",
			id:     fmt.Sprintf("%d", exportImport.ID),
			keyID:  importFilePublicKey.KeyID,
			status: 500,
			want:   []string{"invalid action"},
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

			server := newHTTPServer(t, http.MethodPost, "/:id/:action/:keyid", s.HandleExportImportKeys())

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s/%s/%s", server.URL, tc.id, tc.action, tc.keyID), strings.NewReader(form.Encode()))
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
