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

package publish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/go-cmp/cmp"
)

func TestEnvconfigProcess(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    *publish.Config
		exp      *publish.Config
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
			input: &publish.Config{},
			exp: &publish.Config{
				AuthorizedApp: authorizedapp.Config{
					CacheDuration: 5 * time.Minute,
				},
				Database: database.Config{
					Host:    "localhost",
					Port:    "5432",
					SSLMode: "require",
				},
				SecretManager: secrets.Config{
					SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
					SecretsDir:        "/var/run/secrets",
					SecretCacheTTL:    5 * time.Minute,
				},
				Port:                "8080",
				MinRequestDuration:  5 * time.Second,
				MaxKeysOnPublish:    15,
				MaxIntervalAge:      360 * time.Hour,
				TruncateWindow:      1 * time.Hour,
				DebugAPIResponses:   false,
				DebugAllowRestOfDay: false,
			},
			lookuper: envconfig.MapLookuper(map[string]string{}),
		},
		{
			name:  "values",
			input: &publish.Config{},
			exp: &publish.Config{
				AuthorizedApp: authorizedapp.Config{
					CacheDuration: 10 * time.Minute,
				},
				Database: database.Config{
					Name:               "dbname",
					User:               "dbuser",
					Host:               "https://dbhost",
					Port:               "5555",
					SSLMode:            "verify-ca",
					ConnectionTimeout:  30,
					Password:           "abcd1234",
					SSLCertPath:        "/var/sslcert",
					SSLKeyPath:         "/var/sslkey",
					SSLRootCertPath:    "/var/sslrootcert",
					PoolMinConnections: "5",
					PoolMaxConnections: "50",
					PoolMaxConnLife:    5 * time.Minute,
					PoolMaxConnIdle:    10 * time.Minute,
					PoolHealthCheck:    15 * time.Minute,
				},
				SecretManager: secrets.Config{
					SecretManagerType: secrets.SecretManagerType("NOOP"),
					SecretsDir:        "/tmp",
					SecretCacheTTL:    10 * time.Minute,
				},
				Port:                "5000",
				MinRequestDuration:  1 * time.Second,
				MaxKeysOnPublish:    10,
				MaxIntervalAge:      1 * time.Hour,
				TruncateWindow:      30 * time.Minute,
				DebugAPIResponses:   true,
				DebugAllowRestOfDay: true,
			},
			lookuper: envconfig.MapLookuper(map[string]string{
				"AUTHORIZED_APP_CACHE_DURATION": "10m",

				"DB_NAME":                     "dbname",
				"DB_USER":                     "dbuser",
				"DB_HOST":                     "https://dbhost",
				"DB_PORT":                     "5555",
				"DB_SSLMODE":                  "verify-ca",
				"DB_CONNECT_TIMEOUT":          "30",
				"DB_PASSWORD":                 "abcd1234",
				"DB_SSLCERT":                  "/var/sslcert",
				"DB_SSLKEY":                   "/var/sslkey",
				"DB_SSLROOTCERT":              "/var/sslrootcert",
				"DB_POOL_MIN_CONNS":           "5",
				"DB_POOL_MAX_CONNS":           "50",
				"DB_POOL_MAX_CONN_LIFETIME":   "5m",
				"DB_POOL_MAX_CONN_IDLE_TIME":  "10m",
				"DB_POOL_HEALTH_CHECK_PERIOD": "15m",

				"SECRET_MANAGER":   "NOOP",
				"SECRETS_DIR":      "/tmp",
				"SECRET_CACHE_TTL": "10m",

				"PORT":                        "5000",
				"TARGET_REQUEST_DURATION":     "1s",
				"MAX_KEYS_ON_PUBLISH":         "10",
				"MAX_INTERVAL_AGE_ON_PUBLISH": "1h",
				"TRUNCATE_WINDOW":             "30m",
				"DEBUG_API_RESPONSES":         "true",
				"DEBUG_ALLOW_REST_OF_DAY":     "true",
			}),
		},
		{
			name: "overrides",
			input: &publish.Config{
				AuthorizedApp: authorizedapp.Config{
					CacheDuration: 10 * time.Minute,
				},
				Database: database.Config{
					Name:               "dbname2",
					User:               "dbuser2",
					Host:               "https://dbhost2",
					Port:               "5556",
					SSLMode:            "verify-full",
					ConnectionTimeout:  60,
					Password:           "efgh5678",
					SSLCertPath:        "/var/sslcert2",
					SSLKeyPath:         "/var/sslkey2",
					SSLRootCertPath:    "/var/sslrootcert2",
					PoolMinConnections: "10",
					PoolMaxConnections: "100",
					PoolMaxConnLife:    1 * time.Minute,
					PoolMaxConnIdle:    10 * time.Minute,
					PoolHealthCheck:    100 * time.Minute,
				},
				SecretManager: secrets.Config{
					SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
					SecretsDir:        "/var",
					SecretCacheTTL:    30 * time.Minute,
				},
				Port:                "6000",
				MinRequestDuration:  5 * time.Second,
				MaxKeysOnPublish:    100,
				MaxIntervalAge:      10 * time.Hour,
				TruncateWindow:      300 * time.Minute,
				DebugAPIResponses:   true,
				DebugAllowRestOfDay: true,
			},
			exp: &publish.Config{
				AuthorizedApp: authorizedapp.Config{
					CacheDuration: 10 * time.Minute,
				},
				Database: database.Config{
					Name:               "dbname2",
					User:               "dbuser2",
					Host:               "https://dbhost2",
					Port:               "5556",
					SSLMode:            "verify-full",
					ConnectionTimeout:  60,
					Password:           "efgh5678",
					SSLCertPath:        "/var/sslcert2",
					SSLKeyPath:         "/var/sslkey2",
					SSLRootCertPath:    "/var/sslrootcert2",
					PoolMinConnections: "10",
					PoolMaxConnections: "100",
					PoolMaxConnLife:    1 * time.Minute,
					PoolMaxConnIdle:    10 * time.Minute,
					PoolHealthCheck:    100 * time.Minute,
				},
				SecretManager: secrets.Config{
					SecretManagerType: secrets.SecretManagerType("GOOGLE_SECRET_MANAGER"),
					SecretsDir:        "/var",
					SecretCacheTTL:    30 * time.Minute,
				},
				Port:                "6000",
				MinRequestDuration:  5 * time.Second,
				MaxKeysOnPublish:    100,
				MaxIntervalAge:      10 * time.Hour,
				TruncateWindow:      300 * time.Minute,
				DebugAPIResponses:   true,
				DebugAllowRestOfDay: true,
			},
			lookuper: envconfig.MapLookuper(map[string]string{
				"AUTHORIZED_APP_CACHE_DURATION": "10m",

				"DB_NAME":                     "dbname",
				"DB_USER":                     "dbuser",
				"DB_HOST":                     "https://dbhost",
				"DB_PORT":                     "5555",
				"DB_SSLMODE":                  "verify-ca",
				"DB_CONNECT_TIMEOUT":          "30",
				"DB_PASSWORD":                 "abcd1234",
				"DB_SSLCERT":                  "/var/sslcert",
				"DB_SSLKEY":                   "/var/sslkey",
				"DB_SSLROOTCERT":              "/var/sslrootcert",
				"DB_POOL_MIN_CONNS":           "5",
				"DB_POOL_MAX_CONNS":           "50",
				"DB_POOL_MAX_CONN_LIFETIME":   "5m",
				"DB_POOL_MAX_CONN_IDLE_TIME":  "10m",
				"DB_POOL_HEALTH_CHECK_PERIOD": "15m",

				"SECRET_MANAGER":   "NOOP",
				"SECRETS_DIR":      "/tmp",
				"SECRET_CACHE_TTL": "10m",

				"PORT":                        "5000",
				"TARGET_REQUEST_DURATION":     "1s",
				"MAX_KEYS_ON_PUBLISH":         "10",
				"MAX_INTERVAL_AGE_ON_PUBLISH": "1h",
				"TRUNCATE_WINDOW":             "30m",
				"DEBUG_API_RESPONSES":         "true",
				"DEBUG_ALLOW_REST_OF_DAY":     "true",
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
