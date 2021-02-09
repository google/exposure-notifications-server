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
	"fmt"
	"sort"
	"sync"
	"time"
)

// KeyManager defines the interface for working with a KMS system that
// is able to sign bytes using PKI.
// KeyManager implementations must be able to return a crypto.Signer.
type KeyManager interface {
	NewSigner(ctx context.Context, keyID string) (crypto.Signer, error)

	// Encrypt will encrypt a byte array along with accompanying Additional Authenticated Data (AAD).
	// The ability for AAD to be empty, depends on the implementation being used.
	//
	// Currently Google Cloud KMS, Hashicorp Vault and AWS KMS support AAD
	// The Azure Key Vault implementation does not.
	Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error)

	// Decrypt will decrypt a previously encrypted byte array along with accompanying Additional
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

// KeyManagerFunc is a func that returns a key manager or error.
type KeyManagerFunc func(context.Context, *Config) (KeyManager, error)

// managers is the list of registered key managers.
var managers = make(map[string]KeyManagerFunc)
var managersLock sync.RWMutex

// RegisterManager registers a new key manager with the given name. If a
// manager is already registered with the given name, it panics. Managers are
// usually registered via an init function.
func RegisterManager(name string, fn KeyManagerFunc) {
	managersLock.Lock()
	defer managersLock.Unlock()

	if _, ok := managers[name]; ok {
		panic(fmt.Sprintf("key manager %q is already registered", name))
	}
	managers[name] = fn
}

// RegisteredManagers returns the list of the names of the registered key
// managers.
func RegisteredManagers() []string {
	managersLock.RLock()
	defer managersLock.RUnlock()

	list := make([]string, 0, len(managers))
	for k := range managers {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

// KeyManagerFor returns the key manager with the given name, or an error
// if one does not exist.
func KeyManagerFor(ctx context.Context, cfg *Config) (KeyManager, error) {
	managersLock.RLock()
	defer managersLock.RUnlock()

	name := cfg.Type
	fn, ok := managers[name]
	if !ok {
		return nil, fmt.Errorf("unknown or uncompiled key manager %q", name)
	}
	return fn(ctx, cfg)
}
