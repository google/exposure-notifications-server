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
	"log"
	"strings"
	"time"

	authorizedappdb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/cache"
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

	cache *cache.Cache
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
		cache:         cache.New(),
	}

	// Apply options.
	for _, opt := range opts {
		provider = opt(provider)
	}

	return provider, nil
}

// AppConfig returns the config for the given app package name.
func (p *DatabaseProvider) AppConfig(ctx context.Context, name string) (*model.AuthorizedApp, error) {
	logger := logging.FromContext(ctx)

	// The database treats the app package names as case-insensitive, but our
	// cacher does not. To maximize cache hits, convert to lowercase.
	name = strings.ToLower(name)

	lookup := func() (interface{}, error) {
		// Load config.
		config, err := p.loadAuthorizedAppFromDatabase(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("authorizedapp: %w", err)
		}
		logger.Infof("authorizedapp: loaded %v, caching for %s", name, p.cacheDuration)
		return config, nil
	}
	cached, err := p.cache.WriteThruLookup(name, lookup, p.cacheDuration)

	// Indicates an error on the write thru lookup.
	if err != nil {
		return nil, err
	}

	// Handle not found.
	config := cached.(*model.AuthorizedApp)
	if config == nil {
		return nil, ErrAppNotFound
	}

	log.Printf("AppConfig: %+v %v", config, err)

	// Returned config.
	return config, nil
}

// loadAuthorizedAppFromDatabase is a lower-level private API that actually loads and parses
// a single AuthorizedApp from the database.
func (p *DatabaseProvider) loadAuthorizedAppFromDatabase(ctx context.Context, name string) (*model.AuthorizedApp, error) {
	logger := logging.FromContext(ctx)

	logger.Infof("authorizedapp: loading %v from database", name)
	config, err := authorizedappdb.New(p.database).GetAuthorizedApp(ctx, p.secretManager, name)
	if err != nil {
		return nil, fmt.Errorf("failed to read %v from database: %w", name, err)
	}
	return config, nil
}
