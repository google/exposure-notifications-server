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
	"strings"
	"testing"

	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/pkg/keys"
)

func TestRenderSignatureInfo(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	sigInfo := &model.SignatureInfo{}
	m["siginfo"] = sigInfo

	testRenderTemplate(t, "siginfo", m)
}

func TestHandleSignatureInfoSave(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	exportDB := database.New(db)

	keyManager := env.KeyManager()
	var fileSystemKeys *keys.Filesystem
	switch v := keyManager.(type) {
	case *keys.Filesystem:
		fileSystemKeys = v
	default:
		t.Fatalf("non filesystem key manager installed")
	}
	key, err := fileSystemKeys.CreateSigningKey(ctx, "test/siginfo", "key")
	if err != nil {
		t.Fatalf("failed to create test signing key: %v", err)
	}
	keyVersion, err := fileSystemKeys.CreateKeyVersion(ctx, key)
	if err != nil {
		t.Fatalf("failed to create key version: %v", err)
	}

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
		{
			name: "update_existing",
			seed: &model.SignatureInfo{
				SigningKey:        "/test/case/key/3",
				SigningKeyVersion: "wrong",
				SigningKeyID:      "v43-2",
			},
			form: &signatureInfoFormData{
				SigningKey:        "/test/case/key/3",
				SigningKeyID:      "foo-3",
				SigningKeyVersion: "v42-3",
			},
			want: []string{"foo-3"},
		},
		{
			name: "update_id_mismatch",
			seed: &model.SignatureInfo{
				SigningKey:        "/test/case/key/4",
				SigningKeyVersion: "wrong",
				SigningKeyID:      "v43-4",
			},
			form: &signatureInfoFormData{
				SigningKey:        "/test/case/key/4",
				SigningKeyID:      "foo-4",
				SigningKeyVersion: "v42-4",
			},
			idChange: func(s string) string { return fmt.Sprintf("100%s", s) },
			want:     []string{"error processing signature info", "no rows in result set"},
		},
		{
			name: "invalid_timestamp",
			form: &signatureInfoFormData{
				SigningKey:        "/test/case/key/5",
				SigningKeyID:      "foo",
				SigningKeyVersion: "v42",
				EndDate:           "tomorrow",
				EndTime:           "midnight",
			},
			want: []string{
				"error processing signature info: parsing time",
				"cannot parse",
				"tomorrow midnight",
			},
		},
		{
			name: "sign_hello_world",
			form: &signatureInfoFormData{
				SigningKey:        keyVersion,
				SigningKeyID:      "foo-6",
				SigningKeyVersion: "v42-6",
			},
			want: []string{
				"The signature for the string \"hello world\" for this key is <code>",
				"Updated signture info #",
			},
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
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s", server.URL, id), strings.NewReader(form.Encode()))
			if err != nil {
				t.Fatal(err)
			}
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

	env, s := newTestServer(t)
	db := env.Database()

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
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/%s", server.URL, tc.id), nil)
			if err != nil {
				t.Fatal(err)
			}
			client := server.Client()

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("error making http call: %v", err)
			}

			mustFindStrings(t, resp, tc.want...)
		})
	}
}
