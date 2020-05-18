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
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// DBConfigProvider ensures that the envionment config can provide a DB config.
// All binaries in this application connect to the databse via the same method.
type DBConfigProvider interface {
	DB() *database.Config
}

// AuthorizedAppConfigProvider signals that the config provided knows how to
// configure authorized apps.
type AuthorizedAppConfigProvider interface {
	AuthorizedAppConfig() *authorizedapp.Config
}

// KeyManagerProvider is a marker interface indicating the KeyManagerProvider should be installed.
type KeyManagerProvider interface {
	KeyManager() bool
}

// BlobStorageConfigProvider is a marker interface indicating the BlobStorage interface should be installed.
type BlobStorageConfigProvider interface {
	BlobStorage() bool
}

// Function returned from setup to be deferred until the caller exits.
type Defer func()

// Setup runs common intitializion code for all servers.
func Setup(ctx context.Context, config DBConfigProvider) (*serverenv.ServerEnv, Defer, error) {
	logger := logging.FromContext(ctx)

	// Can be changed with a different secret manager interface.
	// TODO(mikehelmick): Make this extensible to other providers.
	// TODO(sethvargo): Make TTL configurable.
	sm, err := secrets.NewCacher(ctx, secrets.NewGCPSecretManager, 5*time.Minute)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to secret manager: %v", err)
	}
	logger.Infof("Effective environment variables: %+v", config)

	if err := envconfig.Process(ctx, config, sm); err != nil {
		return nil, nil, fmt.Errorf("error loading environment variables: %v", err)
	}

	// Start building serverenv opts
	opts := []serverenv.Option{
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext),
	}

	// TODO(mikehelmick): Make this extensible to other providers.
	if _, ok := config.(KeyManagerProvider); ok {
		km, err := signing.NewGCPKMS(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to connect to key manager: %w", err)
		}
		opts = append(opts, serverenv.WithKeyManager(km))
	}
	// TODO(mikehelmick): Make this extensible to other providers.
	if _, ok := config.(BlobStorageConfigProvider); ok {
		storage, err := storage.NewGoogleCloudStorage(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to connect to storage system: %v", err)
		}
		opts = append(opts, serverenv.WithBlobStorage(storage))
	}

	// Setup the database connection.
	db, err := database.NewFromEnv(ctx, config.DB())
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to database: %v", err)
	}
	{
		// Log the database config, but omit the password field.
		redactedDB := config.DB()
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
