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

package apiconfig

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/secrets"
)

const (
	// defaultCacheDuration is the default amount of time to cache.
	defaultCacheDuration = 5 * time.Minute
)

// Compile-time check to assert implementation.
var _ Provider = (*DatabaseProvider)(nil)

// DatabaseProvider is an Provider that pulls from the database and caches and
// refreshes values on failure.
type DatabaseProvider struct {
	database      *database.DB
	secretManager secrets.SecretManager
	cacheDuration time.Duration

	cache     map[string]*model.APIConfig
	cachedAt  time.Time
	cacheLock sync.RWMutex
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
	logger := logging.FromContext(ctx)

	provider := &DatabaseProvider{
		database:      db,
		cacheDuration: config.CacheDuration,
		cache:         make(map[string]*model.APIConfig),
	}

	// Apply options.
	for _, opt := range opts {
		provider = opt(provider)
	}

	if provider.cacheDuration <= 0 {
		logger.Infof("apiconfig.DatabaseProvider: using default cache duration of %s", defaultCacheDuration)
		provider.cacheDuration = defaultCacheDuration
	}

	return provider, nil
}

// AppConfig returns the config for the given app package name.
func (p *DatabaseProvider) AppConfig(ctx context.Context, name string) (*model.APIConfig, error) {
	logger := logging.FromContext(ctx)

	// Acquire a read lock first, which allows concurrent readers, to check if
	// there's an item in the cache.
	p.cacheLock.RLock()
	if time.Since(p.cachedAt) <= p.cacheDuration {
		val, ok := p.cache[name]
		if !ok {
			p.cacheLock.RUnlock()
			return nil, AppNotFound
		}
		p.cacheLock.RUnlock()
		return val, nil
	}
	p.cacheLock.RUnlock()

	// Acquire a more aggressive lock now because we're about to mutate. However,
	// it's possible that a concurrent routine has already mutated between our
	// read and write locks, so we have to check again.
	p.cacheLock.Lock()
	if time.Since(p.cachedAt) <= p.cacheDuration {
		val, ok := p.cache[name]
		if !ok {
			p.cacheLock.Unlock()
			return nil, AppNotFound
		}
		p.cacheLock.Unlock()
		return val, nil
	}

	// Load configs.
	configs, err := p.loadFromDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiconfig: %w", err)
	}

	// Cache configs.
	logger.Infof("apiconfig: loaded new configurations, caching for %s", p.cacheDuration)
	p.cache = configs
	p.cachedAt = time.Now()

	// Lookup
	val, ok := p.cache[name]
	if !ok {
		p.cacheLock.Unlock()
		return nil, AppNotFound
	}
	p.cacheLock.Unlock()
	return val, nil
}

// loadFromDatabase is a lower-level private API that actually loads and parses
// the APIConfigs from the database.
func (p *DatabaseProvider) loadFromDatabase(ctx context.Context) (map[string]*model.APIConfig, error) {
	logger := logging.FromContext(ctx)

	logger.Info("apiconfig: loading from database")
	configs, err := p.database.ReadAPIConfigs(ctx, p.secretManager)
	if err != nil {
		return nil, fmt.Errorf("failed to read database: %w", err)
	}

	// Construct a map of app name => config.
	m := make(map[string]*model.APIConfig, len(configs))
	for _, config := range configs {
		m[config.AppPackageName] = config
	}
	return m, nil
}
