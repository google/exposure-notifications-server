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

// This package is the primary infected keys upload service.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/api/handlers"
	"github.com/google/exposure-notifications-server/internal/api/publish"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/dbapiconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secretenv"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	envVars := &publish.Environment{}
	err = secretenv.Process(ctx, "", envVars, sm)
	if err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	db, err := database.NewFromEnv(ctx, &envVars.Database)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	cfgProvider, err := dbapiconfig.NewConfigProvider(db, envVars.APIConfigOpts)
	if err != nil {
		logger.Fatalf("unable to create APIConfig provider: %v", err)
	}
	opts := []serverenv.Option{
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext),
		serverenv.WithAPIConfigProvider(cfgProvider),
	}
	env := serverenv.New(ctx, opts...)

	handler, err := publish.NewHandler(ctx, db, env, envVars)
	if err != nil {
		logger.Fatalf("unable to create publish handler: %v", err)
	}
	http.Handle("/", handlers.WithMinimumLatency(envVars.MinRequestDuration, handler))
	logger.Info("starting exposure server")
	log.Fatal(http.ListenAndServe(":"+envVars.Port, nil))
}
