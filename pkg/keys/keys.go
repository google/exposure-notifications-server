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

// Package keys defines the interface to and implementation of key management
// operations.
//
// Although exported, this package is non intended for general consumption. It
// is a shared dependency between multiple exposure notifications projects. We
// cannot guarantee that there won't be breaking changes in the future.
package keys

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// KeyManager defines the interface for working with a KMS system that
// is able to sign bytes using PKI.
// KeyManager implementations must be able to return a crypto.Signer.
type KeyManager interface {
	NewSigner(ctx context.Context, keyID string) (crypto.Signer, error)

	// Encrypt wile enctypt a byte array along with accompaning Additional Authenticated Data (AAD).
	// The ability for AAD to be empty, depends on the implementation being used.
	//
	// Currently Google Cloud KMS, Hashicorp Vault and AWS KMS support AAD
	// The Azure Key Vault implementation does not.
	Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error)

	// Decrypt will descrypt a previously encrypted byte array along with accompaning Additional
	// Authenticated Data (AAD).
	// If AAD was passed in on the encryption, the same AAD must be passed in to decrypt.
	//
	// Currently Google Cloud KMS, Hashicorp Vault and AWS KMS support AAD
	// The Azure Key Vault implementation does not.
	Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad []byte) ([]byte, error)
}

// KeyVersionCreator supports creating a new version of an existing key.
type KeyVersionCreator interface {
	// CreateKeyVersion creates a new key version for the given parent, returning
	// the ID of the new version. The parent key must already exist.
	CreateKeyVersion(ctx context.Context, parent string) (string, error)
}

// KeyVersionDestroyer supports destroying a key version.
type KeyVersionDestroyer interface {
	// DestroyKeyVersion destroys the given key version, if it exists. If the
	// version does not exist, it should not return an error.
	DestroyKeyVersion(ctx context.Context, id string) error
}

// SigningKeyVersion represents the necessary details that this application needs
// to manage signing keys in an external KMS.
type SigningKeyVersion interface {
	KeyID() string
	CreatedAt() time.Time
	DestroyedAt() time.Time
	Signer(ctx context.Context) (crypto.Signer, error)
}

// SigningKeyManager supports extended management of signing keys, versions, and
// rotation.
type SigningKeyManager interface {
	// SigningKeyVersions returns the list of signing keys for the provided
	// parent. If the parent does not exist, it returns an error.
	SigningKeyVersions(ctx context.Context, parent string) ([]SigningKeyVersion, error)

	// CreateSigningKey creates a new signing key in the given parent, returning
	// the id. If the key already exists, it returns the key's id.
	CreateSigningKey(ctx context.Context, parent, name string) (string, error)

	KeyVersionCreator
	KeyVersionDestroyer
}

// EncryptionKeyManager supports extended management of encryption keys,
// versions, and rotation.
type EncryptionKeyManager interface {
	// CreateEncryptionKey creates a new encryption key in the given parent,
	// returning the id. If the key already exists, it returns the key's id.
	CreateEncryptionKey(ctx context.Context, parent, name string) (string, error)

	KeyVersionCreator
	KeyVersionDestroyer
}

// KeyManagerFor returns the appropriate key manager for the given type.
func KeyManagerFor(ctx context.Context, config *Config) (KeyManager, error) {
	typ := config.KeyManagerType
	switch typ {
	case KeyManagerTypeAWSKMS:
		return NewAWSKMS(ctx)
	case KeyManagerTypeAzureKeyVault:
		return NewAzureKeyVault(ctx)
	case KeyManagerTypeGoogleCloudKMS:
		return NewGoogleCloudKMS(ctx, config)
	case KeyManagerTypeHashiCorpVault:
		return NewHashiCorpVault(ctx)
	case KeyManagerTypeFilesystem:
		return NewFilesystem(ctx, config.FilesystemRoot)
	}

	return nil, fmt.Errorf("unknown key manager type: %v", typ)
}

// parsePublicKeyPEM parses the public key in PEM-encoded format into a public
// key.
func parsePublicKeyPEM(s string) (interface{}, error) {
	block, _ := pem.Decode([]byte(s))
	if block == nil {
		return nil, fmt.Errorf("pem is invalid")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key PEM: %w", err)
	}
	return key, nil
}
