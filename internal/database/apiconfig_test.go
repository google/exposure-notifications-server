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

package database

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/model/apiconfig"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type testSecretManager struct {
	values map[string]string
}

func (s *testSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	v, ok := s.values[name]
	if !ok {
		return "", fmt.Errorf("missing %q", name)
	}
	return v, nil
}

func TestReadAPIConfigs(t *testing.T) {
	if testDB == nil {
		t.Skip("no test DB")
	}
	defer resetTestDB(t)
	ctx := context.Background()

	// Create private key for parsing later
	p8PrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	derKey, err := x509.MarshalPKCS8PrivateKey(p8PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derKey,
	})

	sm := &testSecretManager{
		values: map[string]string{
			"team_id":     "ABCD1234",
			"key_id":      "DEFG5678",
			"private_key": string(pemBytes),
		},
	}

	cases := []struct {
		name string
		sql  string
		args []interface{}
		exp  []*apiconfig.APIConfig
		err  bool
	}{
		{
			name: "bare",
			sql: `
				INSERT INTO APIConfig (app_package_name, platform, allowed_regions)
				VALUES ($1, $2, $3)
			`,
			args: []interface{}{"myapp", "ios", []string{"US"}},
			exp: []*apiconfig.APIConfig{
				{
					AppPackageName:  "myapp",
					Platform:        "ios",
					AllowedRegions:  map[string]bool{"US": true},
					CTSProfileMatch: true,
					BasicIntegrity:  true,
				},
			},
		},
		{
			name: "allowed_past_time",
			sql: `
				INSERT INTO APIConfig (
					app_package_name, platform, allowed_regions,
					allowed_past_seconds
				) VALUES ($1, $2, $3, $4)
			`,
			args: []interface{}{"myapp", "ios", []string{"US"}, 1800},
			exp: []*apiconfig.APIConfig{
				{
					AppPackageName:  "myapp",
					Platform:        "ios",
					AllowedRegions:  map[string]bool{"US": true},
					CTSProfileMatch: true,
					BasicIntegrity:  true,
					AllowedPastTime: 30 * time.Minute,
				},
			},
		},
		{
			name: "allowed_future_time",
			sql: `
				INSERT INTO APIConfig (
					app_package_name, platform, allowed_regions,
					allowed_future_seconds
				) VALUES ($1, $2, $3, $4)
			`,
			args: []interface{}{"myapp", "ios", []string{"US"}, 1800},
			exp: []*apiconfig.APIConfig{
				{
					AppPackageName:    "myapp",
					Platform:          "ios",
					AllowedRegions:    map[string]bool{"US": true},
					CTSProfileMatch:   true,
					BasicIntegrity:    true,
					AllowedFutureTime: 30 * time.Minute,
				},
			},
		},
		{
			name: "ios_devicecheck",
			sql: `
				INSERT INTO APIConfig (
					app_package_name, platform, allowed_regions,
					ios_devicecheck_team_id_secret, ios_devicecheck_key_id_secret, ios_devicecheck_private_key_secret
				) VALUES ($1, $2, $3, $4, $5, $6)
			`,
			args: []interface{}{"myapp", "ios", []string{"US"}, "team_id", "key_id", "private_key"},
			exp: []*apiconfig.APIConfig{
				{
					AppPackageName:        "myapp",
					Platform:              "ios",
					AllowedRegions:        map[string]bool{"US": true},
					CTSProfileMatch:       true,
					BasicIntegrity:        true,
					DeviceCheckTeamID:     "ABCD1234",
					DeviceCheckKeyID:      "DEFG5678",
					DeviceCheckPrivateKey: p8PrivateKey,
				},
			},
		},
	}

	for _, c := range cases {
		c := c

		t.Run(c.name, func(t *testing.T) {
			// Acquire a connection
			conn, err := testDB.pool.Acquire(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Release()

			// Insert the data
			if _, err := conn.Exec(ctx, c.sql, c.args...); err != nil {
				t.Fatal(err)
			}

			configs, err := testDB.ReadAPIConfigs(ctx, sm)
			if (err != nil) != c.err {
				t.Fatal(err)
			}

			// Compare, ignoring the private key part
			opts := cmpopts.IgnoreTypes(new(ecdsa.PrivateKey))
			if diff := cmp.Diff(configs, c.exp, opts); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}

			resetTestDB(t)
		})
	}
}
