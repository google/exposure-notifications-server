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

// +build azure all

package keys

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/azurekeyvault"
	"github.com/google/exposure-notifications-server/pkg/base64util"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest"
)

func init() {
	RegisterManager("AZURE_KEY_VAULT", NewAzureKeyVault)
}

// Compile-time check to verify implements interface.
var (
	_ KeyManager    = (*AzureKeyVault)(nil)
	_ crypto.Signer = (*AzureKeyVaultSigner)(nil)
)

// AzureKeyVault implements the keys.KeyManager interface and can be used to
// sign export files.
type AzureKeyVault struct {
	client *keyvault.BaseClient
}

type AZKeyID struct {
	Vault   string
	Key     string
	Version string
}

func ParseAZKeyID(keyID string) (*AZKeyID, error) {
	parts := strings.SplitN(keyID, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("key must include vaultName, keyName, and keyVersion: %v", keyID)
	}

	vault := fmt.Sprintf("https://%s.vault.azure.net", parts[0])
	key, version := parts[1], parts[2]

	return &AZKeyID{Vault: vault, Key: key, Version: version}, nil
}

// NewAzureKeyVault creates a new KeyVault key manager instance.
func NewAzureKeyVault(ctx context.Context, _ *Config) (KeyManager, error) {
	authorizer, err := azurekeyvault.GetKeyVaultAuthorizer()
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

var _ SigningKeyVersion = (*azureKeyVaultKeyVersion)(nil)

type azureKeyVaultKeyVersion struct {
	kv        *AzureKeyVault
	kid       string
	createdAt time.Time
}

func (v *azureKeyVaultKeyVersion) KeyID() string          { return v.kid }
func (v *azureKeyVaultKeyVersion) CreatedAt() time.Time   { return v.createdAt.UTC() }
func (v *azureKeyVaultKeyVersion) DestroyedAt() time.Time { return time.Time{} }
func (v *azureKeyVaultKeyVersion) Signer(ctx context.Context) (crypto.Signer, error) {
	return v.kv.NewSigner(ctx, v.kid)
}

// SigningKeyVersions returns the list of signing keys for the provided
// parent. If the parent does not exist, it returns an error.
func (v *AzureKeyVault) SigningKeyVersions(ctx context.Context, parent string) ([]SigningKeyVersion, error) {
	parts := strings.SplitN(parent, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("key must include vaultName, keyName: %v", parent)
	}

	vaultID := fmt.Sprintf("https://%s.vault.azure.net", parts[0])
	keyID := parts[1]

	maxResults := int32(512)
	resp, err := v.client.GetKeyVersionsComplete(ctx, vaultID, keyID, &maxResults)
	if err != nil {
		return nil, err
	}

	var versions []SigningKeyVersion
	for resp.NotDone() {
		if err := resp.NextWithContext(ctx); err != nil {
			return nil, fmt.Errorf("failed to list: %w", err)
		}

		key := resp.Value()
		versions = append(versions, &azureKeyVaultKeyVersion{
			kv:        v,
			kid:       *key.Kid,
			createdAt: time.Time(*key.Attributes.Created),
		})
	}

	// Sort versions - the results come in as a map, so order is undefined.
	sort.Slice(versions, func(i, j int) bool {
		x := versions[i].(*azureKeyVaultKeyVersion).kid
		y := versions[j].(*azureKeyVaultKeyVersion).kid
		return x < y
	})

	return versions, nil
}

// CreateSigningKey creates a new signing key in the given parent, returning
// the id. If the key already exists, it returns the key's id.
func (v *AzureKeyVault) CreateSigningKey(ctx context.Context, parent, name string) (string, error) {
	vaultID := fmt.Sprintf("https://%s.vault.azure.net", parent)
	if _, err := v.client.GetKey(ctx, vaultID, name, ""); err != nil {
		var aerr autorest.DetailedError
		if errors.As(err, &aerr) {
			// Yea, StatusCode is an interface, so this is what we got...
			if fmt.Sprintf("%d", aerr.StatusCode) == "404" {
				// There's a race here where technically the key could be created between
				// when we checked and now, and there's no CAS in Azure's API, so just...
				// hope?
				return v.CreateKeyVersion(ctx, fmt.Sprintf("%s/%s", parent, name))
			}
		}

		return "", err
	}

	return fmt.Sprintf("%s/%s", parent, name), nil
}

// CreateKeyVersion creates a new key version for the given parent, returning
// the ID of the new version. The parent key must already exist.
func (v *AzureKeyVault) CreateKeyVersion(ctx context.Context, parent string) (string, error) {
	parts := strings.SplitN(parent, "/", 2)
	if len(parts) < 2 {
		return "", fmt.Errorf("key must include vaultName, keyName: %v", parent)
	}

	vaultID := fmt.Sprintf("https://%s.vault.azure.net", parts[0])
	keyID := parts[1]

	resp, err := v.client.CreateKey(ctx, vaultID, keyID, keyvault.KeyCreateParameters{
		Kty:   keyvault.EC,
		Curve: keyvault.P256,
		KeyOps: &[]keyvault.JSONWebKeyOperation{
			keyvault.Sign,
			keyvault.Verify,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	if resp.Key == nil || resp.Key.Kid == nil {
		return "", fmt.Errorf("bad response")
	}

	versionID := path.Base(*resp.Key.Kid)
	return fmt.Sprintf("%s/%s", parent, versionID), nil
}

// DestroyKeyVersion destroys the given key version, if it exists. If the
// version does not exist, it should not return an error.
func (v *AzureKeyVault) DestroyKeyVersion(ctx context.Context, id string) error {
	return fmt.Errorf("keyvault does not support destroying a key version")
}

func (v *AzureKeyVault) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error) {
	k, err := ParseAZKeyID(keyID)
	if err != nil {
		return nil, err
	}

	// TODO(mikehelmick) - neds AEAD
	value := base64.URLEncoding.EncodeToString(plaintext)
	parameters := keyvault.KeyOperationsParameters{
		Algorithm: keyvault.RSAOAEP256,
		Value:     &value,
	}
	res, err := v.client.Encrypt(ctx, k.Vault, k.Key, k.Version, parameters)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt: %w", err)
	}

	resBytes, err := base64.RawURLEncoding.DecodeString(*res.Result)
	if err != nil {
		return nil, fmt.Errorf("unable to decode encrypted data: %w", err)
	}

	return resBytes, nil
}

func (v *AzureKeyVault) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad []byte) ([]byte, error) {
	k, err := ParseAZKeyID(keyID)
	if err != nil {
		return nil, err
	}

	// TODO(mikehelmick) - neds AEAD
	value := base64.URLEncoding.EncodeToString(ciphertext)
	parameters := keyvault.KeyOperationsParameters{
		Algorithm: keyvault.RSAOAEP256,
		Value:     &value,
	}
	res, err := v.client.Decrypt(ctx, k.Vault, k.Key, k.Version, parameters)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt: %w", err)
	}

	plaintext, err := base64.RawURLEncoding.DecodeString(*res.Result)
	if err != nil {
		return nil, fmt.Errorf("unable to decode decrypted data: %w", err)
	}

	return plaintext, nil
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
	k, err := ParseAZKeyID(keyID)
	if err != nil {
		return nil, err
	}
	return NewAzureKeyVaultSigner(ctx, v.client, k.Vault, k.Key, k.Version)
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
