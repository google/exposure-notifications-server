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
	"crypto/ecdsa"
	"fmt"
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

// EncryptionKeyCreator supports creating encryption keys.
type EncryptionKeyCreator interface {
	CreateEncryptionKey(string) ([]byte, error)
}

// EncryptionKeyAdder supports creating encryption keys.
type EncryptionKeyAdder interface {
	AddEncryptionKey(string, []byte) error
}

// SigningKeyCreator supports creating signing keys.
type SigningKeyCreator interface {
	CreateSigningKey(string) (*ecdsa.PrivateKey, error)
}

// SigningKeyAdder supports creating signing keys.
type SigningKeyAdder interface {
	AddSigningKey(string, *ecdsa.PrivateKey) error
}

// KeyManagerFor returns the appropriate key manager for the given type.
func KeyManagerFor(ctx context.Context, typ KeyManagerType) (KeyManager, error) {
	switch typ {
	case KeyManagerTypeAWSKMS:
		return NewAWSKMS(ctx)
	case KeyManagerTypeAzureKeyVault:
		return NewAzureKeyVault(ctx)
	case KeyManagerTypeGoogleCloudKMS:
		return NewGoogleCloudKMS(ctx)
	case KeyManagerTypeHashiCorpVault:
		return NewHashiCorpVault(ctx)
	case KeyManagerTypeInMemory:
		return NewInMemory(ctx)
	}

	return nil, fmt.Errorf("unknown key manager type: %v", typ)
}
