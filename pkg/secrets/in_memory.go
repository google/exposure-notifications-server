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
	"path"
	"strconv"
	"sync"
	"time"
)

func init() {
	RegisterManager("IN_MEMORY", NewInMemory)
}

// Compile-time check to verify implements interface.
var _ SecretVersionManager = (*InMemory)(nil)

// InMemory is an in-memory secret manager, primarily used for testing.
type InMemory struct {
	mu      sync.Mutex
	secrets map[string][]byte
}

// NewInMemory creates a new in-memory secret manager.
func NewInMemory(ctx context.Context, _ *Config) (SecretManager, error) {
	return &InMemory{
		secrets: make(map[string][]byte),
	}, nil
}

// NewInMemoryFromMap creates a new in-memory secret manager from the map.
func NewInMemoryFromMap(ctx context.Context, m map[string]string) (SecretManager, error) {
	if m == nil {
		m = make(map[string]string)
	}

	n := make(map[string][]byte, len(m))
	for k, v := range m {
		n[k] = []byte(v)
	}

	return &InMemory{
		secrets: n,
	}, nil
}

// GetSecretValue returns the secret if it exists, otherwise an error.
func (sm *InMemory) GetSecretValue(_ context.Context, k string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	v, ok := sm.secrets[k]
	if !ok {
		return "", fmt.Errorf("secret does not exist")
	}
	return string(v), nil
}

// CreateSecretVersion creates a new secret version on the given parent with the
// provided data. It returns a reference to the created version.
func (sm *InMemory) CreateSecretVersion(ctx context.Context, parent string, data []byte) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	k := path.Join(parent, version)
	sm.secrets[k] = data
	return k, nil
}

// DestroySecretVersion destroys the secret version with the given name. If the
// version does not exist, no action is taken.
func (sm *InMemory) DestroySecretVersion(ctx context.Context, k string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.secrets, k)
	return nil
}
