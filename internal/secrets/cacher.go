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
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
)

// Compile-time check to verify implements interface.
var _ SecretManager = (*Cacher)(nil)

// Cacher is a secret manager implementation that wraps another secret manager
// and caches secret values.
type Cacher struct {
	sm  SecretManager
	ttl time.Duration

	cache      map[string]*cachedItem
	cacheMutex sync.Mutex
}

type cachedItem struct {
	value    string
	cachedAt time.Time
}

// NewCacher creates a new secret manager that caches results for the given ttl.
func NewCacher(ctx context.Context, f SecretManagerFunc, ttl time.Duration) (SecretManager, error) {
	sm, err := f(ctx)
	if err != nil {
		return nil, fmt.Errorf("cacher: %w", err)
	}

	return WrapCacher(ctx, sm, ttl), nil
}

// WrapCacher wraps an existing SecretManager with caching.
func WrapCacher(ctx context.Context, sm SecretManager, ttl time.Duration) SecretManager {
	return &Cacher{
		sm:    sm,
		ttl:   ttl,
		cache: make(map[string]*cachedItem),
	}
}

// GetSecretValue implements the SecretManager interface, but caches values and
// retrieves them from the cache.
func (sm *Cacher) GetSecretValue(ctx context.Context, name string) (string, error) {
	logger := logging.FromContext(ctx)

	// Lock
	sm.cacheMutex.Lock()
	defer sm.cacheMutex.Unlock()

	// Lookup in cache
	if i, ok := sm.cache[name]; ok && time.Since(i.cachedAt) < sm.ttl {
		logger.Debugf("loaded secret %v from cache", name)
		return i.value, nil
	}

	// Delegate lookup to parent sm.
	plaintext, err := sm.sm.GetSecretValue(ctx, name)
	if err != nil {
		return "", err
	}

	// Cache value
	sm.cache[name] = &cachedItem{
		value:    plaintext,
		cachedAt: time.Now(),
	}

	return plaintext, nil
}
