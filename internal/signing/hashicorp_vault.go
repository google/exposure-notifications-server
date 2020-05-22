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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/google/exposure-notifications-server/internal/base64util"
	vaultapi "github.com/hashicorp/vault/api"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*HashiCorpVault)(nil)
var _ crypto.Signer = (*HashiCorpVaultSigner)(nil)

// HashiCorpVault implements the signing.KeyManager interface and can be used to
// sign export files.
type HashiCorpVault struct {
	client *vaultapi.Client
}

// NewHashiCorpVault creates a new Vault key manager instance.
func NewHashiCorpVault(ctx context.Context) (KeyManager, error) {
	client, err := vaultapi.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("secrets.NewHashiCorpVault: client: %w", err)
	}

	sm := &HashiCorpVault{
		client: client,
	}

	return sm, nil
}

// NewSigner creates a new signer that uses a key in HashiCorp Vault's transit
// backend. The keyID is in the format:
//
//     name@version
//
// Both name and version are required.
func (v *HashiCorpVault) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	parts := strings.SplitN(keyID, "@", 2)
	switch len(parts) {
	case 0, 1:
		return nil, fmt.Errorf("missing version in: %v", keyID)
	default:
		return NewHashiCorpVaultSigner(ctx, v.client, parts[0], parts[1])
	}
}

type HashiCorpVaultSigner struct {
	client  *vaultapi.Client
	name    string
	version string

	publicKey crypto.PublicKey
}

// NewHashiCorpVaultSigner creates a new signing interface compatiable with
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

	publicKeyPemRaw, ok := keyTyped["public_key"]
	if !ok {
		return nil, fmt.Errorf("no public_key field")
	}

	publicKeyPem, ok := publicKeyPemRaw.(string)
	if !ok {
		return nil, fmt.Errorf("invalid public_key type %T", publicKeyPemRaw)
	}

	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil {
		return nil, fmt.Errorf("pem is invalid")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key pem: %w", err)
	}

	typed, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("invalid public key type: %T", publicKey)
	}

	return typed, nil
}
