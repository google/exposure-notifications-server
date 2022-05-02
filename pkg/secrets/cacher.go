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

package secrets

import (
	"context"
	"time"

	"github.com/google/exposure-notifications-server/pkg/cache"
)

// Compile-time check to verify implements interface.
var _ SecretManager = (*Cacher)(nil)

// Cacher is a secret manager implementation that wraps another secret manager
// and caches secret values.
type Cacher struct {
	sm    SecretManager
	cache *cache.Cache[string]
}

// WrapCacher wraps an existing SecretManager with caching.
func WrapCacher(ctx context.Context, sm SecretManager, ttl time.Duration) (SecretManager, error) {
	cache, err := cache.New[string](ttl)
	if err != nil {
		return nil, err
	}
	return &Cacher{
		sm:    sm,
		cache: cache,
	}, nil
}

// GetSecretValue implements the SecretManager interface, but caches values and
// retrieves them from the cache.
func (sm *Cacher) GetSecretValue(ctx context.Context, name string) (string, error) {
	lookup := func() (string, error) {
		// Delegate lookup to parent sm.
		plaintext, err := sm.sm.GetSecretValue(ctx, name)
		if err != nil {
			return "", err
		}
		return plaintext, nil
	}

	plaintext, err := sm.cache.WriteThruLookup(name, lookup)
	if err != nil {
		return "", err
	}

	return plaintext, nil
}
