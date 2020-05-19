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

package authorizedapp

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/secrets"
)

// Compile-time check to assert implementation.
var _ Provider = (*DatabaseProvider)(nil)

// DatabaseProvider is a Provider that pulls from the database and caches and
// refreshes values on failure.
type DatabaseProvider struct {
	database      *database.DB
	secretManager secrets.SecretManager
	cacheDuration time.Duration

	cache     map[string]*cacheItem
	cacheLock sync.RWMutex
}

type cacheItem struct {
	value    *database.AuthorizedApp
	cachedAt time.Time
}

// DatabaseProviderOption is used as input to the database provider.
type DatabaseProviderOption func(*DatabaseProvider) *DatabaseProvider

// WithSecretManager sets the secret manager for resolving secrets.
func WithSecretManager(sm secrets.SecretManager) DatabaseProviderOption {
	return func(p *DatabaseProvider) *DatabaseProvider {
		p.secretManager = sm
		return p
	}
}

// NewDatabaseProvider creates a new Provider that reads from a database.
func NewDatabaseProvider(ctx context.Context, db *database.DB, config *Config, opts ...DatabaseProviderOption) (Provider, error) {
	provider := &DatabaseProvider{
		database:      db,
		cacheDuration: config.CacheDuration,
		cache:         make(map[string]*cacheItem),
	}

	// Apply options.
	for _, opt := range opts {
		provider = opt(provider)
	}

	return provider, nil
}

// checkCache checks the local cache within a read lock.
// The bool on return is true if there was a hit (And an error is a valid hit)
// or false if there was a miss (or expiry) and the data source should be queried again.
func (p *DatabaseProvider) checkCache(name string) (*database.AuthorizedApp, bool, error) {
	// Acquire a read lock first, which allows concurrent readers, to check if
	// there's an item in the cache.
	p.cacheLock.RLock()
	defer p.cacheLock.RUnlock()

	item, ok := p.cache[name]
	if ok && time.Since(item.cachedAt) <= p.cacheDuration {
		if item.value == nil {
			return nil, true, AppNotFound
		}
		return item.value, true, nil
	}
	return nil, false, nil
}

// AppConfig returns the config for the given app package name.
func (p *DatabaseProvider) AppConfig(ctx context.Context, name string) (*database.AuthorizedApp, error) {
	logger := logging.FromContext(ctx)

	data, cacheHit, error := p.checkCache(name)
	if cacheHit {
		return data, error
	}

	// Acquire a more aggressive lock now because we're about to mutate. However,
	// it's possible that a concurrent routine has already mutated between our
	// read and write locks, so we have to check again.
	p.cacheLock.Lock()
	defer p.cacheLock.Unlock()
	item, ok := p.cache[name]
	if ok && time.Since(item.cachedAt) <= p.cacheDuration {
		if item.value == nil {
			return nil, AppNotFound
		}
		return item.value, nil
	}

	// Load config.
	config, err := p.loadAuthorizedAppFromDatabase(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("authorizedapp: %w", err)
	}

	// Cache configs.
	logger.Infof("authorizedapp: loaded %v, caching for %s", name, p.cacheDuration)
	p.cache[name] = &cacheItem{
		value:    config,
		cachedAt: time.Now(),
	}

	// Handle not found.
	if config == nil {
		return nil, AppNotFound
	}

	// Returned config.
	return config, nil
}

// loadAuthorizedAppFromDatabase is a lower-level private API that actually loads and parses
// a single AuthorizedApp from the database.
func (p *DatabaseProvider) loadAuthorizedAppFromDatabase(ctx context.Context, name string) (*database.AuthorizedApp, error) {
	logger := logging.FromContext(ctx)

	logger.Infof("authorizedapp: loading %v from database", name)
	config, err := database.NewAuthorizedAppDB(p.database).GetAuthorizedApp(ctx, p.secretManager, name)
	if err != nil {
		return nil, fmt.Errorf("failed to read %v from database: %w", name, err)
	}
	return config, nil
}
