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

package keys

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"fmt"
	"math/big"
	"strings"
	"testing"

	vaultlog "github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	vaulttransit "github.com/hashicorp/vault/builtin/logical/transit"
	vaulthttp "github.com/hashicorp/vault/http"
	vaultlogical "github.com/hashicorp/vault/sdk/logical"
	vault "github.com/hashicorp/vault/vault"
)

func TestNewHashiCorpVaultSigner(t *testing.T) {
	cases := []struct {
		name       string
		client     *vaultapi.Client
		keyName    string
		keyVersion string
	}{
		{
			name:   "no_client",
			client: nil,
		},
		{
			name:    "no_name",
			client:  new(vaultapi.Client),
			keyName: "",
		},
		{
			name:       "no_version",
			client:     new(vaultapi.Client),
			keyName:    "foobar",
			keyVersion: "",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := NewHashiCorpVaultSigner(ctx, tc.client, tc.keyName, tc.keyVersion)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestHashiCorpVaultEncryptDecrypt(t *testing.T) {
	setupFn := func(client *vaultapi.Client) error {
		if _, err := client.Logical().Write("transit/keys/my-key", map[string]interface{}{
			"type": "aes256-gcm96",
		}); err != nil {
			return fmt.Errorf("failed to create key: %w", err)
		}
		return nil
	}
	identityFn := func(b []byte) []byte {
		return b
	}

	cases := []struct {
		name  string
		setup func(client *vaultapi.Client) error
		keyID string

		plaintext    string
		aad          string
		modifyCipher func(b []byte) []byte
		modifyAAD    func(b []byte) []byte

		err    bool
		errMsg string
	}{
		{
			name:         "correct",
			setup:        setupFn,
			keyID:        "my-key@1",
			plaintext:    "this is where you put secret stuff",
			aad:          "pizza",
			modifyCipher: identityFn,
			modifyAAD:    identityFn,
			err:          false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a Vault server.
			ctx := context.Background()
			core, _, token := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
				DisableMlock: true,
				DisableCache: true,
				Logger:       vaultlog.NewNullLogger(),
				LogicalBackends: map[string]vaultlogical.Factory{
					"transit": vaulttransit.Factory,
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

			// Enable transit.
			if _, err := client.Logical().Write("sys/mounts/transit", map[string]interface{}{
				"type": "transit",
			}); err != nil {
				t.Fatal(err)
			}

			// Run setup.
			if err := tc.setup(client); err != nil {
				t.Fatal(err)
			}

			// Wrap client.
			vault := &HashiCorpVault{client}

			ciphertext, err := vault.Encrypt(ctx, tc.keyID, []byte(tc.plaintext), tc.aad)
			if err != nil {
				t.Fatal(err)
			}

			decrypted, err := vault.Decrypt(ctx, tc.keyID, ciphertext, tc.aad)
			if err != nil {
				t.Fatal(err)
			}

			if tc.plaintext != string(decrypted) {
				t.Fatalf("encrypt/decrypt got different result: want: %v got: %v", tc.plaintext, string(decrypted))
			}
		})
	}
}

func TestHashiCorpVaultSigner_Public(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(client *vaultapi.Client) error
		keyName    string
		keyVersion string

		err    bool
		errMsg string
	}{
		{
			name: "correct",
			setup: func(client *vaultapi.Client) error {
				if _, err := client.Logical().Write("transit/keys/my-key", map[string]interface{}{
					"type": "ecdsa-p256",
				}); err != nil {
					return fmt.Errorf("failed to create key: %w", err)
				}
				return nil
			},
			keyName:    "my-key",
			keyVersion: "1",
		},
		{
			name: "bad_version",
			setup: func(client *vaultapi.Client) error {
				if _, err := client.Logical().Write("transit/keys/my-key", map[string]interface{}{
					"type": "ecdsa-p256",
				}); err != nil {
					return fmt.Errorf("failed to create key: %w", err)
				}
				return nil
			},
			keyName:    "my-key",
			keyVersion: "23",
			err:        true,
			errMsg:     "has no version 23",
		},
		{
			name: "wrong_key_type_rsa",
			setup: func(client *vaultapi.Client) error {
				if _, err := client.Logical().Write("transit/keys/my-key", map[string]interface{}{
					"type": "rsa-4096",
				}); err != nil {
					return fmt.Errorf("failed to create key: %w", err)
				}
				return nil
			},
			keyName:    "my-key",
			keyVersion: "1",
			err:        true,
			errMsg:     "invalid key type",
		},
		{
			name: "not_exist",
			setup: func(client *vaultapi.Client) error {
				return nil
			},
			keyName:    "my-key",
			keyVersion: "1",
			err:        true,
			errMsg:     "public key was empty",
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			// Create a Vault server.
			ctx := context.Background()
			core, _, token := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
				DisableMlock: true,
				DisableCache: true,
				Logger:       vaultlog.NewNullLogger(),
				LogicalBackends: map[string]vaultlogical.Factory{
					"transit": vaulttransit.Factory,
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

			// Enable transit.
			if _, err := client.Logical().Write("sys/mounts/transit", map[string]interface{}{
				"type": "transit",
			}); err != nil {
				t.Fatal(err)
			}

			// Run setup.
			if err := tc.setup(client); err != nil {
				t.Fatal(err)
			}

			// Create signer.
			signer, err := NewHashiCorpVaultSigner(ctx, client, tc.keyName, tc.keyVersion)
			if err != nil {
				if !tc.err {
					t.Fatal(err)
				}

				if !strings.Contains(err.Error(), tc.errMsg) {
					t.Errorf("expected %q to contain %q", err.Error(), tc.errMsg)
				}
			}

			if signer != nil {
				pub := signer.Public()
				if _, ok := pub.(*ecdsa.PublicKey); !ok {
					t.Errorf("expected public key to be ecdsa, was %T", pub)
				}
			}
		})
	}
}

func TestHashiCorpVaultSigner_Sign(t *testing.T) {
	// Create a Vault server.
	ctx := context.Background()
	core, _, token := vault.TestCoreUnsealedWithConfig(t, &vault.CoreConfig{
		DisableMlock: true,
		DisableCache: true,
		Logger:       vaultlog.NewNullLogger(),
		LogicalBackends: map[string]vaultlogical.Factory{
			"transit": vaulttransit.Factory,
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

	// Enable transit.
	if err := client.Sys().Mount("transit/", &vaultapi.MountInput{
		Type: "transit",
	}); err != nil {
		t.Fatal(err)
	}

	// Create the key.
	if _, err := client.Logical().Write("transit/keys/my-key", map[string]interface{}{
		"type": "ecdsa-p256",
	}); err != nil {
		t.Fatal(err)
	}

	// Create the signer.
	signer, err := NewHashiCorpVaultSigner(ctx, client, "my-key", "1")
	if err != nil {
		panic(err)
	}

	// Generate data and digest.
	data := []byte("why hello there!")
	digest := sha256.Sum256(data)

	// Sign!
	sig, err := signer.Sign(rand.Reader, digest[:], crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	rs := struct {
		R, S *big.Int
	}{}
	if _, err := asn1.Unmarshal(sig, &rs); err != nil {
		t.Fatal(err)
	}

	// Verify.
	pub, ok := signer.Public().(*ecdsa.PublicKey)
	if !ok {
		t.Errorf("%T is not *ecdsa.PublicKey", pub)
	}

	if ok := ecdsa.Verify(pub, digest[:], rs.R, rs.S); !ok {
		t.Errorf("expected ok")
	}
}
