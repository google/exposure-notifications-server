// Copyright 2020 the Exposure Notifications Server authors
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

//go:build vault || all

package keys

import (
	"context"
	"crypto"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/pkg/base64util"
	"github.com/mitchellh/mapstructure"

	vaultapi "github.com/hashicorp/vault/api"
)

func init() {
	RegisterManager("HASHICORP_VAULT", NewHashiCorpVault)
}

// Compile-time check to verify implements interface.
var (
	_ KeyManager        = (*HashiCorpVault)(nil)
	_ SigningKeyManager = (*HashiCorpVault)(nil)
	_ crypto.Signer     = (*HashiCorpVaultSigner)(nil)
)

// HashiCorpVault implements the keys.KeyManager interface and can be used to
// sign export files and encrypt/decrypt data.
//
// For encryption keys, when using value, the keys must be created with
//   `derived=true`
type HashiCorpVault struct {
	client *vaultapi.Client
}

// NewHashiCorpVault creates a new Vault key manager instance.
func NewHashiCorpVault(ctx context.Context, _ *Config) (KeyManager, error) {
	client, err := vaultapi.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("secrets.NewHashiCorpVault: client: %w", err)
	}

	sm := &HashiCorpVault{
		client: client,
	}

	return sm, nil
}

type HCValueKeyID struct {
	Name    string
	Version string
}

func NewHCValueKeyID(keyID string) (*HCValueKeyID, error) {
	parts := strings.SplitN(keyID, "@", 2)
	switch len(parts) {
	case 0, 1:
		return nil, fmt.Errorf("missing version in: %v", keyID)
	default:
		return &HCValueKeyID{Name: parts[0], Version: parts[1]}, nil
	}
}

type readKeyResponse struct {
	Versions map[string]struct {
		CreationTime time.Time `ms:"creation_time"`
		PublicKeyPEM string    `ms:"public_key"`
	} `ms:"keys"`
}

var _ SigningKeyVersion = (*vaultKeyVersion)(nil)

type vaultKeyVersion struct {
	client    *vaultapi.Client
	name      string
	version   int64
	createdAt time.Time
	publicKey crypto.PublicKey
}

func (v *vaultKeyVersion) KeyID() string          { return fmt.Sprintf("%s/%d", v.name, v.version) }
func (v *vaultKeyVersion) CreatedAt() time.Time   { return v.createdAt.UTC() }
func (v *vaultKeyVersion) DestroyedAt() time.Time { return time.Time{} }
func (v *vaultKeyVersion) Signer(ctx context.Context) (crypto.Signer, error) {
	return &HashiCorpVaultSigner{
		client:    v.client,
		name:      v.name,
		version:   strconv.FormatInt(v.version, 10),
		publicKey: v.publicKey,
	}, nil
}

// SigningKeyVersions returns the signing keys for the given key name.
func (v *HashiCorpVault) SigningKeyVersions(ctx context.Context, parent string) ([]SigningKeyVersion, error) {
	pth := fmt.Sprintf("transit/keys/%s", parent)
	result, err := v.client.Logical().Read(pth)
	if err != nil {
		return nil, fmt.Errorf("unable to read key: %w", err)
	}
	if result == nil || result.Data == nil {
		return nil, fmt.Errorf("key does not exist")
	}

	var response readKeyResponse
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook:       mapstructure.StringToTimeHookFunc(time.RFC3339),
		WeaklyTypedInput: true,
		Result:           &response,
		TagName:          "ms",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to setup decoder: %w", err)
	}
	if err := dec.Decode(result.Data); err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}

	versions := make([]SigningKeyVersion, 0, len(response.Versions))
	for k, ver := range response.Versions {
		publicKey, err := ParseECDSAPublicKey(ver.PublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to parse public key for %s/%s", k, ver)
		}

		num, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid key key %q: %w", k, err)
		}

		versions = append(versions, &vaultKeyVersion{
			client:    v.client,
			name:      parent,
			version:   num,
			createdAt: ver.CreationTime,
			publicKey: publicKey,
		})
	}

	// Sort versions - the results come in as a map, so order is undefined.
	sort.Slice(versions, func(i, j int) bool {
		x := versions[i].(*vaultKeyVersion).version
		y := versions[j].(*vaultKeyVersion).version
		return x < y
	})

	return versions, nil
}

// CreateSigningKey creates a new signing key with the given name.
func (v *HashiCorpVault) CreateSigningKey(ctx context.Context, parent, name string) (string, error) {
	id := strings.Trim(strings.Join([]string{parent, name}, "/"), "/")
	pth := fmt.Sprintf("/transit/keys/%s", id)
	if _, err := v.client.Logical().Write(pth, map[string]interface{}{
		"name": name,
		"type": "ecdsa-p256",
	}); err != nil {
		return "", fmt.Errorf("failed to create signing key: %w", err)
	}

	return id, nil
}

// CreateKeyVersion rotates the given key.
func (v *HashiCorpVault) CreateKeyVersion(ctx context.Context, parent string) (string, error) {
	pth := fmt.Sprintf("/transit/keys/%s/rotate", parent)
	if _, err := v.client.Logical().Write(pth, nil); err != nil {
		return "", fmt.Errorf("failed to rotate signing key: %w", err)
	}

	// WARNING: race condition. Since Vault's API does not return information
	// about the key that was just rotated, we have to READ the API to get the
	// latest version. If another version was created between ours and now, we'd
	// pick up that new version.
	//
	// To put it another way, there's no guarantee that the last element in this
	// result is the element we just created.
	list, err := v.SigningKeyVersions(ctx, parent)
	if err != nil {
		return "", fmt.Errorf("failed to lookup signing keys: %w", err)
	}

	if len(list) < 1 {
		return "", fmt.Errorf("no signing keys")
	}
	return list[len(list)-1].KeyID(), nil
}

// DestroyKeyVersion is unimplemented on Vault. Vault can only trim keys up to a
// point (which might be unsafe).
func (v *HashiCorpVault) DestroyKeyVersion(ctx context.Context, id string) error {
	return fmt.Errorf("vault does not support destroying a key version")
}

// NewSigner creates a new signer that uses a key in HashiCorp Vault's transit
// backend. The keyID is in the format:
//
//     name@version
//
// Both name and version are required.
func (v *HashiCorpVault) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	hvcKey, err := NewHCValueKeyID(keyID)
	if err != nil {
		return nil, err
	}
	return NewHashiCorpVaultSigner(ctx, v.client, hvcKey.Name, hvcKey.Version)
}

func (v *HashiCorpVault) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error) {
	kid, err := NewHCValueKeyID(keyID)
	if err != nil {
		return nil, err
	}
	pth := fmt.Sprintf("transit/encrypt/%s", kid.Name)
	params := map[string]interface{}{
		"plaintext": base64.StdEncoding.EncodeToString(plaintext),
		"context":   base64.StdEncoding.EncodeToString(aad),
		"type":      "aes256-gcm96",
	}

	result, err := v.client.Logical().Write(pth, params)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt: %w", err)
	}

	ciphertext, ok := result.Data["ciphertext"]
	if !ok {
		return nil, fmt.Errorf("encryption returned no ciphertext")
	}

	return []byte(ciphertext.(string)), nil
}

func (v *HashiCorpVault) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad []byte) ([]byte, error) {
	kid, err := NewHCValueKeyID(keyID)
	if err != nil {
		return nil, err
	}
	pth := fmt.Sprintf("transit/decrypt/%s", kid.Name)
	params := map[string]interface{}{
		"ciphertext": string(ciphertext),
		"context":    base64.StdEncoding.EncodeToString(aad),
	}

	result, err := v.client.Logical().Write(pth, params)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt: %w", err)
	}

	plaintext, ok := result.Data["plaintext"]
	if !ok {
		return nil, fmt.Errorf("dencryption returned no plaintext")
	}

	decoded, err := base64util.DecodeString(plaintext.(string))
	if err != nil {
		return nil, fmt.Errorf("unable to decode plaintext: %w", err)
	}

	return decoded, nil
}

type HashiCorpVaultSigner struct {
	client  *vaultapi.Client
	name    string
	version string

	publicKey crypto.PublicKey
}

// NewHashiCorpVaultSigner creates a new signing interface compatible with
// HashiCorp Vault's transit backend. The key name and key version are required.
func NewHashiCorpVaultSigner(ctx context.Context, client *vaultapi.Client, name, version string) (*HashiCorpVaultSigner, error) {
	if client == nil {
		return nil, fmt.Errorf("missing client")
	}

	if name == "" {
		return nil, fmt.Errorf("missing name")
	}

	if version == "" {
		return nil, fmt.Errorf("version is required")
	}

	signer := &HashiCorpVaultSigner{
		client:  client,
		name:    name,
		version: version,
	}

	publicKey, err := signer.getPublicKey()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	signer.publicKey = publicKey

	return signer, nil
}

// Public returns the public key. The public key is fetched when the signer is
// created.
func (s *HashiCorpVaultSigner) Public() crypto.PublicKey {
	return s.publicKey
}

// Sign signs the given digest using the public key.
func (s *HashiCorpVaultSigner) Sign(_ io.Reader, digest []byte, _ crypto.SignerOpts) ([]byte, error) {
	pth := fmt.Sprintf("transit/sign/%s/sha2-256", s.name)
	secret, err := s.client.Logical().Write(pth, map[string]interface{}{
		"input":                base64.StdEncoding.EncodeToString(digest),
		"prehashed":            true,
		"marshaling_algorithm": "asn1",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("got response for signing, but was nil")
	}

	// Check if the "value" key is present.
	raw, ok := secret.Data["signature"]
	if !ok {
		return nil, fmt.Errorf("response does not have 'signature' key")
	}

	signature, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("signature is not a string")
	}

	// Vault returns the signature in the format vault:vX:BASE_64_SIG, extract
	// the raw SIG.
	parts := strings.SplitN(signature, ":", 3)
	actualSignature := parts[len(parts)-1]

	b, err := base64util.DecodeString(actualSignature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	return b, nil
}

func (s *HashiCorpVaultSigner) getPublicKey() (crypto.PublicKey, error) {
	pth := fmt.Sprintf("transit/keys/%s", s.name)
	secret, err := s.client.Logical().Read(pth)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key for %v: %w", s.name, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("found %v, but public key was empty", s.name)
	}

	rawType, ok := secret.Data["type"]
	if !ok {
		return nil, fmt.Errorf("missing type field")
	}

	typ, ok := rawType.(string)
	if !ok {
		return nil, fmt.Errorf("type is not a string")
	}

	if got, want := typ, "ecdsa-p256"; got != want {
		return nil, fmt.Errorf("invalid key type %v: expected %v", got, want)
	}

	rawKeys, ok := secret.Data["keys"]
	if !ok {
		return nil, fmt.Errorf("%v does not contain public keys", s.name)
	}

	m, ok := rawKeys.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%v is not in the correct format: %T", s.name, rawKeys)
	}

	keyRaw, ok := m[s.version]
	if !ok {
		return nil, fmt.Errorf("%v has no version %v", s.name, s.version)
	}

	keyTyped, ok := keyRaw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("not in the correct format: %T", keyRaw)
	}

	publicKeyPEMRaw, ok := keyTyped["public_key"]
	if !ok {
		return nil, fmt.Errorf("no public_key field")
	}

	publicKeyPEM, ok := publicKeyPEMRaw.(string)
	if !ok {
		return nil, fmt.Errorf("invalid public_key type %T", publicKeyPEMRaw)
	}

	return ParseECDSAPublicKey(publicKeyPEM)
}
