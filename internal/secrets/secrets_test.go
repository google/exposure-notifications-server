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

package secrets_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/go-cmp/cmp"
)

func TestEnvconfigProcess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    *secrets.Config
		exp      *secrets.Config
		lookuper envconfig.Lookuper
		err      error
	}{
		{
			name:     "nil",
			lookuper: envconfig.MapLookuper(map[string]string{}),
			err:      envconfig.ErrNotStruct,
		},
		{
			name:  "defaults",
			input: &secrets.Config{},
			exp: &secrets.Config{
				SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
				SecretsDir:        "/var/run/secrets",
				SecretCacheTTL:    5 * time.Minute,
			},
			lookuper: envconfig.MapLookuper(map[string]string{}),
		},
		{
			name:  "values",
			input: &secrets.Config{},
			exp: &secrets.Config{
				SecretManagerType: secrets.SecretManagerType("NOOP"),
				SecretsDir:        "/tmp",
				SecretCacheTTL:    2 * time.Hour,
			},
			lookuper: envconfig.MapLookuper(map[string]string{
				"SECRET_MANAGER":   "NOOP",
				"SECRETS_DIR":      "/tmp",
				"SECRET_CACHE_TTL": "2h",
			}),
		},
		{
			name: "overrides",
			input: &secrets.Config{
				SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
				SecretsDir:        "/var",
				SecretCacheTTL:    1 * time.Hour,
			},
			exp: &secrets.Config{
				SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
				SecretsDir:        "/var",
				SecretCacheTTL:    1 * time.Hour,
			},
			lookuper: envconfig.MapLookuper(map[string]string{
				"SECRET_MANAGER":   "NOOP",
				"SECRETS_DIR":      "/tmp",
				"SECRET_CACHE_TTL": "2h",
			}),
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			if err := envconfig.ProcessWith(ctx, tc.input, tc.lookuper); !errors.Is(err, tc.err) {
				t.Fatalf("expected \n%#v\n to be \n%#v\n", err, tc.err)
			}

			if diff := cmp.Diff(tc.exp, tc.input); diff != "" {
				t.Fatalf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}
