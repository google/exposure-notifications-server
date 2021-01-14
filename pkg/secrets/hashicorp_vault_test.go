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

package secrets

import (
	"context"
	"fmt"
	"strings"
	"testing"

	vaultlog "github.com/hashicorp/go-hclog"
	vaultkv "github.com/hashicorp/vault-plugin-secrets-kv"
	vaultapi "github.com/hashicorp/vault/api"
	vaulthttp "github.com/hashicorp/vault/http"
	vaultlogical "github.com/hashicorp/vault/sdk/logical"
	vault "github.com/hashicorp/vault/vault"
)

func TestHashiCorpVault_GetSecretValue(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(client *vaultapi.Client) error
		secret string
		exp    string
		err    bool
		errMsg string
	}{
		{
			name: "no_exists",
			setup: func(client *vaultapi.Client) error {
				return nil
			},
			secret: "kv/data/my-secret",
			err:    true,
			errMsg: "data is nil",
		},
		{
			name: "no_value",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"not_value": "hello",
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			err:    true,
			errMsg: "missing 'value' key",
		},
		{
			name: "versioned",
			setup: func(client *vaultapi.Client) error {
				for i := 1; i <= 5; i++ {
					if _, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
						"data": map[string]interface{}{
							"value": fmt.Sprintf("hello-%d", i),
						},
					}); err != nil {
						return err
					}
				}
				return nil
			},
			secret: "kv/data/my-secret?version=3",
			exp:    "hello-3",
		},
		{
			name: "string",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"value": "hello",
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			exp:    "hello",
		},
		{
			name: "byte",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"value": byte(123),
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			exp:    "123",
		},
		{
			name: "bool",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"value": true,
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			exp:    "true",
		},
		{
			name: "uint",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"value": uint(5),
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			exp:    "5",
		},
		{
			name: "other",
			setup: func(client *vaultapi.Client) error {
				_, err := client.Logical().Write("kv/data/my-secret", map[string]interface{}{
					"data": map[string]interface{}{
						"value": map[string]interface{}{
							"foo": "bar",
						},
					},
				})
				return err
			},
			secret: "kv/data/my-secret",
			err:    true,
			errMsg: "is of unknown type",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Create a Vault server.
			ctx := context.Background()
			core, _, token := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
				DisableMlock: true,
				DisableCache: true,
				Logger:       vaultlog.NewNullLogger(),
				LogicalBackends: map[string]vaultlogical.Factory{
					"kv": vaultkv.Factory,
				},
			})
			ln, addr := vaulthttp.TestServer(t, core)
			defer ln.Close()

			// Create the client.
			client, err := vaultapi.NewClient(&vaultapi.Config{Address: addr})
			if err != nil {
				t.Fatal(err)
			}
			client.SetToken(token)

			// Enable KVv2.
			if err := client.Sys().Mount("kv/", &vaultapi.MountInput{
				Type: "kv-v2",
			}); err != nil {
				t.Fatal(err)
			}

			// Run setup.
			if err := tc.setup(client); err != nil {
				t.Fatal(err)
			}

			// Create the secrets client.
			secrets := &HashiCorpVault{client: client}

			val, err := secrets.GetSecretValue(ctx, tc.secret)
			if err != nil {
				if !tc.err {
					t.Fatal(err)
				}

				if got, want := err.Error(), tc.errMsg; !strings.Contains(got, want) {
					t.Errorf("expected %q to matc %q", got, want)
				}
			}

			if got, want := val, tc.exp; got != want {
				t.Errorf("expected %q to be %q", got, want)
			}
		})
	}
}
