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
	"fmt"
)

// Compile-time check to verify implements interface.
var _ KeyManager = (*Noop)(nil)

// Noop is a key manager that does nothing and always returns an error.
type Noop struct{}

func NewNoop(ctx context.Context) (KeyManager, error) {
	return &Noop{}, nil
}

func (n *Noop) NewSigner(ctx context.Context, keyID string) (crypto.Signer, error) {
	return nil, fmt.Errorf("noop cannot sign")
}

func (n *Noop) Encrypt(ctx context.Context, keyID string, plaintext []byte, aad string) ([]byte, error) {
	return plaintext, nil
}

func (n *Noop) Decrypt(ctx context.Context, keyID string, ciphertext []byte, aad string) ([]byte, error) {
	return ciphertext, nil
}
