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

package setup

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"

	kenvconfig "github.com/kelseyhightower/envconfig"
)

// BlobstoreConfigProvider provides the information about current storage
// configuration.
type BlobstoreConfigProvider interface {
	BlobstoreConfig() *storage.Config
}

// DatabaseConfigProvider ensures that the environment config can provide a DB config.
// All binaries in this application connect to the database via the same method.
type DatabaseConfigProvider interface {
	DatabaseConfig() *database.Config
}

// AuthorizedAppConfigProvider signals that the config provided knows how to
// configure authorized apps.
type AuthorizedAppConfigProvider interface {
	AuthorizedAppConfig() *authorizedapp.Config
}

// KeyManagerConfigProvider is a marker interface indicating the key manager
// should be installed.
type KeyManagerConfigProvider interface {
	KeyManagerConfig() *signing.Config
}

// SecretManagerConfigProvider signals that the config knows how to configure a
// secret manager.
type SecretManagerConfigProvider interface {
	SecretManagerConfig() *secrets.Config
}

// Function returned from setup to be deferred until the caller exits.
type Defer func()

// Setup runs common initialization code for all servers.
func Setup(ctx context.Context, config DatabaseConfigProvider) (*serverenv.ServerEnv, Defer, error) {
	logger := logging.FromContext(ctx)

	// Load the secret manager - this needs to be loaded first because other
	// processors may require access to secrets.
	var sm secrets.SecretManager
	if provider, ok := config.(SecretManagerConfigProvider); ok {
		// The environment configuration defines which secret manager to use, so we
		// need to process the envconfig in here. Once we know which secret manager
		// to use, we can load the secret manager and then re-process (later) any
		// secret:// references.
		//
		// NOTE: it is not possible to specify which secret manager to use via a
		// secret:// reference. This configuration option must always be the
		// plaintext string.
		smConfig := provider.SecretManagerConfig()
		if err := kenvconfig.Process("", smConfig); err != nil {
			return nil, nil, fmt.Errorf("unable to process secret manager environment: %w", err)
		}

		var err error
		sm, err = secrets.SecretManagerFor(ctx, smConfig.SecretManagerType)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to connect to secret manager: %w", err)
		}

		// Enable caching, if provided.
		if ttl := smConfig.SecretCacheTTL; ttl > 0 {
			sm = secrets.WrapCacher(ctx, sm, ttl)
		}
	}

	// Process first round of environment variables.
	if err := envconfig.Process(ctx, config, sm); err != nil {
		return nil, nil, fmt.Errorf("error loading environment variables: %w", err)
	}
	logger.Infof("Effective environment variables: %+v", config)

	// Start building serverenv opts
	opts := []serverenv.Option{
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext),
	}

	// Configure key management for signing.
	if provider, ok := config.(KeyManagerConfigProvider); ok {
		kmConfig := provider.KeyManagerConfig()
		km, err := signing.KeyManagerFor(ctx, kmConfig.KeyManagerType)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to connect to key manager: %w", err)
		}
		opts = append(opts, serverenv.WithKeyManager(km))
	}

	// Configure blob storage.
	if provider, ok := config.(BlobstoreConfigProvider); ok {
		bsConfig := provider.BlobstoreConfig()
		blobStore, err := storage.BlobstoreFor(ctx, bsConfig.BlobstoreType)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to connect to storage system: %v", err)
		}
		blobStorage := serverenv.WithBlobStorage(blobStore)
		opts = append(opts, blobStorage)
	}

	// Setup the database connection.
	db, err := database.NewFromEnv(ctx, config.DatabaseConfig())
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	{
		// Log the database config, but omit the password field.
		redactedDB := config.DatabaseConfig()
		redactedDB.Password = "<hidden>"
		logger.Infof("Effective DB config: %+v", redactedDB)
	}
	opts = append(opts, serverenv.WithDatabase(db))

	// AuthorizedApp must come after database setup due to the dependency.
	if typ, ok := config.(AuthorizedAppConfigProvider); ok {
		logger.Infof("Effective AuthorizedApp config: %+v", typ.AuthorizedAppConfig())
		provider, err := authorizedapp.NewDatabaseProvider(ctx, db, typ.AuthorizedAppConfig(), authorizedapp.WithSecretManager(sm))
		if err != nil {
			// Ensure the database is closed on an error.
			defer db.Close(ctx)
			return nil, nil, fmt.Errorf("unable to create AuthorizedApp provider: %v", err)
		}
		opts = append(opts, serverenv.WithAuthorizedAppProvider(provider))
	}

	return serverenv.New(ctx, opts...), func() { db.Close(ctx) }, nil
}
