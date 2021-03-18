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
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/project"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestRenderHealthAuthority(t *testing.T) {
	t.Parallel()

	m := TemplateMap{}
	ha := new(model.HealthAuthority)
	hak := new(model.HealthAuthorityKey)
	m["ha"] = ha
	m["hak"] = hak

	testRenderTemplate(t, "healthauthority", m)
}

func TestPopulateHealthAuthority(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		form *healthAuthorityFormData
		exp  *model.HealthAuthority
		err  string
	}{
		{
			name: "default",
			form: &healthAuthorityFormData{
				Issuer:         "test-iss",
				Audience:       "test-aud",
				Name:           "test-ha",
				EnableStatsAPI: true,
				JwksURI:        "https://foo.bar",
			},
			exp: &model.HealthAuthority{
				Issuer:         "test-iss",
				Audience:       "test-aud",
				Name:           "test-ha",
				EnableStatsAPI: true,
				JwksURI:        stringPtr("https://foo.bar"),
			},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var ha model.HealthAuthority
			tc.form.PopulateHealthAuthority(&ha)

			opts := cmp.Options{cmpopts.EquateApproxTime(5 * time.Minute)}
			if diff := cmp.Diff(tc.exp, &ha, opts); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestPopulateHealthAuthorityKey(t *testing.T) {
	t.Parallel()

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	cases := []struct {
		name string
		form *keyhealthAuthorityFormData
		exp  *model.HealthAuthorityKey
		err  string
	}{
		{
			name: "default",
			form: &keyhealthAuthorityFormData{
				Version: "123",
				PEMBlock: `
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEml59itec9qzwVojreLXdPNRsUWzf
YHc1cKvIIi6/H56AJS/kZEYQnfDpxrgyGhdAm+pNN2GAJ3XdnQZ1Sk4amg==
-----END PUBLIC KEY-----
`,
				FromDate: "2021-01-02",
				FromTime: "09:23",
				ThruDate: "2022-01-02",
				ThruTime: "10:34",
			},
			exp: &model.HealthAuthorityKey{
				Version: "123",
				From:    from,
				Thru:    thru,
				PublicKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEml59itec9qzwVojreLXdPNRsUWzf
YHc1cKvIIi6/H56AJS/kZEYQnfDpxrgyGhdAm+pNN2GAJ3XdnQZ1Sk4amg==
-----END PUBLIC KEY-----`,
			},
		},
		{
			name: "bad_from",
			form: &keyhealthAuthorityFormData{
				FromDate: "banana",
				FromTime: "apple",
			},
			err: "invalid from time",
		},
		{
			name: "bad_thru",
			form: &keyhealthAuthorityFormData{
				ThruDate: "banana",
				ThruTime: "apple",
			},
			err: "invalid thru time",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var key model.HealthAuthorityKey
			err := tc.form.PopulateHealthAuthorityKey(&key)
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

func TestHandleHealthAuthorityShow(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	verificationDB := database.New(db)

	healthAuthority := &model.HealthAuthority{
		Issuer:         "test-iss",
		Audience:       "test-aud",
		Name:           "TEST",
		Keys:           []*model.HealthAuthorityKey{},
		JwksURI:        stringPtr("https://foo.bar"),
		EnableStatsAPI: true,
	}
	if err := verificationDB.AddHealthAuthority(ctx, healthAuthority); err != nil {
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
			want:   []string{"New Verification Key"},
		},
		{
			name:   "existing",
			id:     fmt.Sprintf("%d", healthAuthority.ID),
			status: 200,
			want:   []string{"Edit Verification Key", healthAuthority.Issuer, healthAuthority.Audience},
		},
		{
			name:   "bad_id",
			id:     "banana",
			status: 500,
			want:   []string{"Unable to parse `id` param"},
		},
		{
			name:   "non_existing",
			id:     "123456",
			status: 500,
			want:   []string{"Unable to find requested health authority"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			server := newHTTPServer(t, http.MethodGet, "/:id", s.HandleHealthAuthorityShow())

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

func TestHandleHealthAuthoritySave(t *testing.T) {
	t.Parallel()
	ctx := project.TestContext(t)

	env, s := newTestServer(t)
	db := env.Database()
	verificationDB := database.New(db)

	healthAuthority := &model.HealthAuthority{
		Issuer:         "test-iss",
		Audience:       "test-aud",
		Name:           "TEST",
		Keys:           []*model.HealthAuthorityKey{},
		JwksURI:        stringPtr("https://foo.bar"),
		EnableStatsAPI: true,
	}
	if err := verificationDB.AddHealthAuthority(ctx, healthAuthority); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		id     string
		seed   *model.HealthAuthority
		form   *healthAuthorityFormData
		status int
		want   []string
	}{
		// create
		{
			name: "create_new",
			id:   "0",
			form: &healthAuthorityFormData{
				Issuer:         "test-iss1",
				Audience:       "test-aud1",
				Name:           "test-ha1",
				EnableStatsAPI: true,
				JwksURI:        "https://foo.bar",
			},
			status: 303,
		},

		// update
		{
			name:   "update_unknown",
			id:     "123456",
			form:   &healthAuthorityFormData{},
			status: 500,
			want:   []string{"error processing health authority"},
		},
		{
			name:   "update_bad_id",
			id:     "banana",
			form:   &healthAuthorityFormData{},
			status: 500,
			want:   []string{"failed to parse"},
		},
		{
			name:   "update_request",
			id:     fmt.Sprintf("%d", healthAuthority.ID),
			form:   &healthAuthorityFormData{},
			status: 500,
			want:   []string{"Error writing health authority"},
		},
		{
			name: "update_existing",
			id:   fmt.Sprintf("%d", healthAuthority.ID),
			form: &healthAuthorityFormData{
				Issuer:         "test-iss2",
				Audience:       "test-aud2",
				Name:           "test-ha2",
				EnableStatsAPI: true,
				JwksURI:        "https://foo.bar",
			},
			status: 303,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := verificationDB.AddHealthAuthority(ctx, tc.seed); err != nil {
					t.Fatalf("failed to add health authority: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id", s.HandleHealthAuthoritySave())

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

func TestHandleHealthAuthorityKeys(t *testing.T) {
	t.Parallel()

	ctx := project.TestContext(t)

	from := time.Unix(1609579380, 0).UTC()
	thru := time.Unix(1641119640, 0).UTC()

	env, s := newTestServer(t)
	db := env.Database()
	verificationDB := database.New(db)

	healthAuthority := &model.HealthAuthority{
		Issuer:         "test-iss",
		Audience:       "test-aud",
		Name:           "TEST",
		Keys:           []*model.HealthAuthorityKey{},
		JwksURI:        stringPtr("https://foo.bar"),
		EnableStatsAPI: true,
	}
	if err := verificationDB.AddHealthAuthority(ctx, healthAuthority); err != nil {
		t.Fatal(err)
	}

	healthAuthorityKey := &model.HealthAuthorityKey{
		AuthorityID: healthAuthority.ID,
		Version:     "123",
		From:        from,
		Thru:        thru,
		PublicKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEml59itec9qzwVojreLXdPNRsUWzf
YHc1cKvIIi6/H56AJS/kZEYQnfDpxrgyGhdAm+pNN2GAJ3XdnQZ1Sk4amg==
-----END PUBLIC KEY-----`,
	}
	if err := verificationDB.AddHealthAuthorityKey(ctx, healthAuthority, healthAuthorityKey); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		action  string
		id      string
		version string
		seed    *model.HealthAuthorityKey
		form    *keyhealthAuthorityFormData
		status  int
		want    []string
	}{
		{
			name:    "invalid_id",
			action:  "create",
			id:      "banana",
			version: "0",
			status:  500,
			want:    []string{"Unable to parse `id` param"},
		},
		{
			name:    "not_existing",
			action:  "create",
			id:      "123",
			version: "0",
			status:  500,
			want:    []string{"error processing health authority"},
		},

		// create
		{
			name:    "create_new",
			action:  "create",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: "0",
			form: &keyhealthAuthorityFormData{
				Version: "key_version2",
				PEMBlock: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEml59itec9qzwVojreLXdPNRsUWzf
YHc1cKvIIi6/H56AJS/kZEYQnfDpxrgyGhdAm+pNN2GAJ3XdnQZ1Sk4amg==
-----END PUBLIC KEY-----`,
				FromDate: "2021-01-02",
				FromTime: "09:23",
				ThruDate: "2022-01-02",
				ThruTime: "10:34",
			},
			status: 303,
		},
		{
			name:    "create_validation",
			action:  "create",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: "0",
			form: &keyhealthAuthorityFormData{
				Version:  "key_version",
				PEMBlock: "PEMPEMPEM",
				FromDate: "bad-date",
				FromTime: "bad-time",
				ThruDate: "bad-date",
				ThruTime: "bad-time",
			},
			status: 500,
			want:   []string{"invalid from time"},
		},

		// revoke
		{
			name:    "revoke",
			action:  "revoke",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: healthAuthorityKey.Version,
			status:  303,
		},
		{
			name:    "revoke_not_exist",
			action:  "revoke",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: "not-a-real-key-to-find",
			status:  500,
			want:    []string{"Invalid key specified"},
		},

		// reinstate
		{
			name:    "reinstate",
			action:  "reinstate",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: healthAuthorityKey.Version,
			status:  303,
		},

		// activate
		{
			name:    "activate",
			action:  "activate",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: healthAuthorityKey.Version,
			status:  303,
		},

		// unknown
		{
			name:    "invalid_action",
			action:  "nope",
			id:      fmt.Sprintf("%d", healthAuthority.ID),
			version: healthAuthorityKey.Version,
			status:  500,
			want:    []string{"invalid action"},
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.seed != nil {
				if err := verificationDB.AddHealthAuthorityKey(ctx, healthAuthority, tc.seed); err != nil {
					t.Fatalf("failed to add health authority key: %v", err)
				}
			}

			server := newHTTPServer(t, http.MethodPost, "/:id/:action/:version", s.HandleHealthAuthorityKeys())

			// URL values
			form, err := serializeForm(tc.form)
			if err != nil {
				t.Fatalf("unable to serialize form: %v", err)
			}

			ctx := context.Background()
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/%s/%s/%s", server.URL, tc.id, tc.action, tc.version), strings.NewReader(form.Encode()))
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
