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
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/sethvargo/go-envconfig/pkg/envconfig"
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

// Setup runs common initialization code for all servers. See SetupWith.
func Setup(ctx context.Context, config interface{}) (*serverenv.ServerEnv, error) {
	return SetupWith(ctx, config, envconfig.OsLookuper())
}

// SetupWith processes the given configuration using envconfig. It is
// responsible for establishing database connections, resolving secrets, and
// accessing app configs. The provided interface must implement the various
// interfaces.
func SetupWith(ctx context.Context, config interface{}, l envconfig.Lookuper) (*serverenv.ServerEnv, error) {
	logger := logging.FromContext(ctx)

	// Build a list of mutators. This list will grow as we initialize more of the
	// configuration, such as the secret manager.
	var mutatorFuncs []envconfig.MutatorFunc

	// Build a list of options to pass to the server env.
	var serverEnvOpts []serverenv.Option

	// TODO: support customizable metrics
	serverEnvOpts = append(serverEnvOpts, serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext))

	// Load the secret manager - this needs to be loaded first because other
	// processors may require access to secrets.
	var sm secrets.SecretManager
	if provider, ok := config.(SecretManagerConfigProvider); ok {
		logger.Info("configuring secret manager")

		// The environment configuration defines which secret manager to use, so we
		// need to process the envconfig in here. Once we know which secret manager
		// to use, we can load the secret manager and then re-process (later) any
		// secret:// references.
		//
		// NOTE: it is not possible to specify which secret manager to use via a
		// secret:// reference. This configuration option must always be the
		// plaintext string.
		smConfig := provider.SecretManagerConfig()
		if err := envconfig.ProcessWith(ctx, smConfig, l, mutatorFuncs...); err != nil {
			return nil, fmt.Errorf("unable to process secret manager env: %w", err)
		}

		var err error
		sm, err = secrets.SecretManagerFor(ctx, smConfig.SecretManagerType)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to secret manager: %w", err)
		}

		// Enable caching, if a TTL was provided.
		if ttl := smConfig.SecretCacheTTL; ttl > 0 {
			sm, err = secrets.WrapCacher(ctx, sm, ttl)
			if err != nil {
				return nil, fmt.Errorf("unable to create secret manager cache: %w", err)
			}
		}

		// Update the mutators to process secrets.
		mutatorFuncs = append(mutatorFuncs, secrets.Resolver(sm, smConfig))

		// Update serverEnv setup.
		serverEnvOpts = append(serverEnvOpts, serverenv.WithSecretManager(sm))

		logger.Infow("secret manager", "config", smConfig)
	}

	// Load the key manager.
	var km signing.KeyManager
	if provider, ok := config.(KeyManagerConfigProvider); ok {
		logger.Info("configuring key manager")

		kmConfig := provider.KeyManagerConfig()
		if err := envconfig.ProcessWith(ctx, kmConfig, l, mutatorFuncs...); err != nil {
			return nil, fmt.Errorf("unable to process key manager env: %w", err)
		}

		var err error
		km, err = signing.KeyManagerFor(ctx, kmConfig.KeyManagerType)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to key manager: %w", err)
		}

		// Update serverEnv setup.
		serverEnvOpts = append(serverEnvOpts, serverenv.WithKeyManager(km))

		logger.Infow("key manager", "config", kmConfig)
	}

	// Process first round of environment variables.
	if err := envconfig.ProcessWith(ctx, config, l, mutatorFuncs...); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}
	logger.Infow("provided", "config", config)

	// Configure blob storage.
	if provider, ok := config.(BlobstoreConfigProvider); ok {
		logger.Info("configuring blobstore")

		bsConfig := provider.BlobstoreConfig()
		blobStore, err := storage.BlobstoreFor(ctx, bsConfig.BlobstoreType)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to storage system: %v", err)
		}
		blobStorage := serverenv.WithBlobStorage(blobStore)

		// Update serverEnv setup.
		serverEnvOpts = append(serverEnvOpts, blobStorage)

		logger.Infow("blobstore", "config", bsConfig)
	}

	// Setup the database connection.
	if provider, ok := config.(DatabaseConfigProvider); ok {
		logger.Info("configuring database")

		dbConfig := provider.DatabaseConfig()
		db, err := database.NewFromEnv(ctx, dbConfig)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to database: %v", err)
		}

		// Update serverEnv setup.
		serverEnvOpts = append(serverEnvOpts, serverenv.WithDatabase(db))

		logger.Infow("database", "config", dbConfig)

		// AuthorizedApp must come after database setup due to the dependency.
		if provider, ok := config.(AuthorizedAppConfigProvider); ok {
			logger.Info("configuring authorizedapp")

			aaConfig := provider.AuthorizedAppConfig()
			aa, err := authorizedapp.NewDatabaseProvider(ctx, db, aaConfig, authorizedapp.WithSecretManager(sm))
			if err != nil {
				// Ensure the database is closed on an error.
				defer db.Close(ctx)
				return nil, fmt.Errorf("unable to create AuthorizedApp provider: %v", err)
			}

			// Update serverEnv setup.
			serverEnvOpts = append(serverEnvOpts, serverenv.WithAuthorizedAppProvider(aa))

			logger.Infow("authorizedapp", "config", aaConfig)
		}
	}

	return serverenv.New(ctx, serverEnvOpts...), nil
}
