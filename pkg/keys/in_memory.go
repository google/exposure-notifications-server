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
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io"
	"sync"
)

// InMemory is useful for testing. Do NOT use in a running system as all
// keys are only kept in memory and will be lost across server reboots.
type InMemory struct {
	mu          sync.RWMutex
	signingKeys map[string]*ecdsa.PrivateKey
	cryptoKeys  map[string][]byte
}

// NewInMemory creates a new, local, in memory KeyManager.
func NewInMemory(ctx context.Context) (*InMemory, error) {
	return &InMemory{
		signingKeys: make(map[string]*ecdsa.PrivateKey),
		cryptoKeys:  make(map[string][]byte),
	}, nil
}

// AddSigningKey generates a new ECDSA P256 Signing Key identified by
// the provided keyID
func (k *InMemory) AddSigningKey(keyID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, ok := k.signingKeys[keyID]; ok {
		return fmt.Errorf("key already exists: %v", keyID)
	}

	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("unable to generate private key: %w", err)
	}

	k.signingKeys[keyID] = pk
	return nil
}

// AddEncryptionKey generates a new encryption key identified by
// the provided keyID.
func (k *InMemory) AddEncryptionKey(keyID string) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if _, ok := k.cryptoKeys[keyID]; ok {
		return fmt.Errorf("key already exists: %v", keyID)
	}

	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("failed to read random bytes: %w", err)
	}

	k.cryptoKeys[keyID] = key

	return nil
}

func (k *InMemory) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	var pk *ecdsa.PrivateKey
	{
		k.mu.RLock()
		defer k.mu.RUnlock()

		if k, ok := k.signingKeys[keyID]; !ok {
			return nil, fmt.Errorf("key not found")
		} else {
			pk = k
		}
	}

	return pk, nil
}

func (k *InMemory) getDEK(keyID string) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	if dek, ok := k.cryptoKeys[keyID]; ok {
		return dek, nil
	}
	return nil, fmt.Errorf("key not found")
}

func (k *InMemory) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad []byte) ([]byte, error) {
	dek, err := k.getDEK(keyID)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("bad cipher block: %w", err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap cipher block: %w", err)
	}
	nonce := make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := aesgcm.Seal(nonce, nonce, plaintext, aad)

	return ciphertext, nil
}

func (k *InMemory) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad []byte) ([]byte, error) {
	dek, err := k.getDEK(keyID)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher from dek: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create gcm from dek: %w", err)
	}

	size := aesgcm.NonceSize()
	if len(ciphertext) < size {
		return nil, fmt.Errorf("malformed ciphertext")
	}
	nonce, ciphertextPortion := ciphertext[:size], ciphertext[size:]

	plaintext, err := aesgcm.Open(nil, nonce, ciphertextPortion, aad)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext with dek: %w", err)
	}

	return plaintext, nil
}
