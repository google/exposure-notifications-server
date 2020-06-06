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

package signing

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"encoding/base64"
	"fmt"
	"io"
	"math/big"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/azure-service-operator/pkg/resourcemanager/config"
	"github.com/Azure/azure-service-operator/pkg/resourcemanager/iam"
	"github.com/google/exposure-notifications-server/internal/base64util"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*AzureKeyVault)(nil)
var _ crypto.Signer = (*AzureKeyVaultSigner)(nil)

// AzureKeyVault implements the signing.KeyManager interface and can be used to
// sign export files.
type AzureKeyVault struct {
	client *keyvault.BaseClient
}

// NewAzureKeyVault creates a new KeyVault key manager instance.
func NewAzureKeyVault(ctx context.Context) (KeyManager, error) {
	err := config.ParseEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to parse env for config: err: %w", err)
	}

	authorizer, err := iam.GetKeyvaultAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("secrets.NewAzureKeyVault: auth: %w", err)
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	sm := &AzureKeyVault{
		client: &client,
	}

	return sm, nil
}

// NewSigner creates a new signer that uses a key in HashiCorp Vault's transit
// backend. The keyID in the format:
//
//     AZURE_KEY_VAULT_NAME/SECRET_NAME/SECRET_VERSION
//
// For example:
//
//     my-company-vault/api-key/1
//
// Both name and version are required.
func (v *AzureKeyVault) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	parts := strings.SplitN(keyID, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("key must include vaultName, keyName, and keyVersion: %v", keyID)
	}

	vault := fmt.Sprintf("https://%s.vault.azure.net", parts[0])
	key, version := parts[1], parts[2]
	return NewAzureKeyVaultSigner(ctx, v.client, vault, key, version)
}

type AzureKeyVaultSigner struct {
	client  *keyvault.BaseClient
	vault   string
	key     string
	version string

	publicKey *ecdsa.PublicKey
}

// NewAzureKeyVaultSigner creates a new signing interface compatible with
// HashiCorp Vault's transit backend. The key name and key version are required.
func NewAzureKeyVaultSigner(ctx context.Context, client *keyvault.BaseClient, vault, key, version string) (*AzureKeyVaultSigner, error) {
	if client == nil {
		return nil, fmt.Errorf("missing client")
	}

	if vault == "" {
		return nil, fmt.Errorf("missing vault")
	}

	if key == "" {
		return nil, fmt.Errorf("missing key")
	}

	if version == "" {
		return nil, fmt.Errorf("missing version")
	}

	signer := &AzureKeyVaultSigner{
		client:  client,
		vault:   vault,
		key:     key,
		version: version,
	}

	publicKey, err := signer.getPublicKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	signer.publicKey = publicKey

	return signer, nil
}

// Public returns the public key. The public key is fetched when the signer is
// created.
func (s *AzureKeyVaultSigner) Public() crypto.PublicKey {
	return s.publicKey
}

// Sign signs the given digest using the public key.
func (s *AzureKeyVaultSigner) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	ctx := context.Background()

	// base64-urlencode the digest
	b64Digest := base64.RawURLEncoding.EncodeToString(digest)

	result, err := s.client.Sign(ctx, s.vault, s.key, s.version, keyvault.KeySignParameters{
		Algorithm: keyvault.ES256,
		Value:     &b64Digest,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	sig := result.Result
	if sig == nil {
		return nil, fmt.Errorf("signature is nil")
	}

	b, err := base64util.DecodeString(*sig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Many hours
	b, err = convert1363ToAsn1(b)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to asn1: %w", err)
	}
	return b, nil
}

func (s *AzureKeyVaultSigner) getPublicKey(ctx context.Context) (*ecdsa.PublicKey, error) {
	bundle, err := s.client.GetKey(ctx, s.vault, s.key, s.version)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %v from %v: %w", s.key, s.vault, err)
	}

	jsonKey := bundle.Key
	if jsonKey == nil {
		return nil, fmt.Errorf("found %v, but it is not a key", s.key)
	}

	if jsonKey.Kty != keyvault.EC {
		return nil, fmt.Errorf("found %v, but type is not EC: %v", s.key, jsonKey.Kty)
	}

	if jsonKey.Crv != keyvault.P256 {
		return nil, fmt.Errorf("found %v, but curve is not P256: %v", s.key, jsonKey.Crv)
	}

	// parse x
	if jsonKey.X == nil {
		return nil, fmt.Errorf("found %v, but X is nil", s.key)
	}
	xRaw, err := base64util.DecodeString(*jsonKey.X)
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode X: %w", err)
	}
	var x big.Int
	x.SetBytes(xRaw)

	// parse y
	if jsonKey.Y == nil {
		return nil, fmt.Errorf("found %v, but Y is nil", s.key)
	}
	yRaw, err := base64util.DecodeString(*jsonKey.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to base64-decode Y: %w", err)
	}
	var y big.Int
	y.SetBytes(yRaw)

	// Final sanity check
	if ok := elliptic.P256().IsOnCurve(&x, &y); !ok {
		return nil, fmt.Errorf("not on curve")
	}

	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     &x,
		Y:     &y,
	}, nil
}

// convert1363ToAsn1 converts a DER-encoded certificate from IEEE 1363 to ASN.1,
// so that they will work with openssl and match the other signature formats.
func convert1363ToAsn1(b []byte) ([]byte, error) {
	rs := struct {
		R, S *big.Int
	}{
		R: new(big.Int).SetBytes(b[:len(b)/2]),
		S: new(big.Int).SetBytes(b[len(b)/2:]),
	}

	return asn1.Marshal(rs)
}
