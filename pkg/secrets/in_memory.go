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

package secrets

import (
	"context"
	"fmt"
)

// Compile-time check to verify implements interface.
var _ SecretManager = (*InMemory)(nil)

// InMemory is an in-memory secret manager, primarily used for testing.
type InMemory struct {
	secrets map[string]string
}

// NewInMemory creates a new in-memory secret manager.
func NewInMemory(ctx context.Context) (SecretManager, error) {
	return &InMemory{
		secrets: make(map[string]string),
	}, nil
}

// NewInMemoryFromMap creates a new in-memory secret manager from the map.
func NewInMemoryFromMap(ctx context.Context, m map[string]string) (SecretManager, error) {
	if m == nil {
		m = make(map[string]string)
	}

	return &InMemory{
		secrets: m,
	}, nil
}

// GetSecretValue returns the secret if it exists, otherwise an error.
func (m *InMemory) GetSecretValue(_ context.Context, k string) (string, error) {
	v, ok := m.secrets[k]
	if !ok {
		return "", fmt.Errorf("secret does not exist")
	}
	return v, nil
}
