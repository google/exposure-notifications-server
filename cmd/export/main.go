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

// This package is the service that publishes infected keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/api/export"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	envVars := &export.Environment{}
	err := envconfig.Process("exposure", envVars)
	if err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	// It is possible to install a different secret management, KMS, and blob
	// storage systems here.
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %w", err)
	}
	km, err := signing.NewGCPKMS(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to key manager: %w", err)
	}
	storage, err := storage.NewGoogleCloudStorage(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to storage system: %v", err)
	}
	// Construct desired serving environment.
	env := serverenv.New(ctx,
		serverenv.WithSecretManager(sm),
		serverenv.WithKeyManager(km),
		serverenv.WithBlobStorage(storage),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext))

	db, err := database.NewFromEnv(ctx, &envVars.Database)
	if err != nil {
		logger.Fatalf("unable to connect to database: %w", err)
	}
	defer db.Close(ctx)

	batchServer, err := export.NewBatchServer(db, envVars, env)
	if err != nil {
		logger.Fatalf("unable to create server: %v", err)
	}
	http.HandleFunc("/create-batches", batchServer.CreateBatchesHandler) // controller that creates work items
	http.HandleFunc("/do-work", batchServer.WorkerHandler)               // worker that executes work

	logger.Info("starting exposure export server")
	log.Fatal(http.ListenAndServe(":"+env.Port, nil))
}
