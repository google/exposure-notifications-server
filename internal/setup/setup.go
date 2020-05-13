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

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/dbapiconfig"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

// DBConfigProvider ensures that the envionment config can provide a DB config.
// All binaries in this application connect to the databse via the same method.
type DBConfigProvider interface {
	DB() *database.Config
}

// DBAPIConfigProvider signals that the config provided knows how to configure
// and requires a DB backed APIConfigProvider installed.
type DBAPIConfigProvider interface {
	API() *dbapiconfig.ConfigOpts
}

// Setup runs common intitializion code for all servers.
func Setup(ctx context.Context, config DBConfigProvider) *serverenv.ServerEnv {
	logger := logging.FromContext(ctx)

	// Can be changed with a different secret manager interface.
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	if err := envconfig.Process(ctx, config, sm); err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	db, err := database.NewFromEnv(ctx, config.DB())
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	//defer db.Close(ctx)

	// Start building serverenv opts
	opts := []serverenv.Option{
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext),
		serverenv.WithPostgresDatabase(db),
		serverenv.WithConfig(config),
	}

	if apicfg, ok := config.(DBAPIConfigProvider); ok {
		logger.Infof("Installing DB APIConfig Provider")
		cfgProvider, err := dbapiconfig.NewConfigProvider(db, apicfg.API())
		if err != nil {
			logger.Fatalf("unable to create APIConfig provider: %v", err)
		}
		opts = append(opts, serverenv.WithAPIConfigProvider(cfgProvider))
	}

	return serverenv.New(ctx, opts...)
}
