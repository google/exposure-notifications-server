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
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/export"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

const (
	createBatchesTimeoutEnvVar = "CREATE_BATCHES_TIMEOUT"
	workerTimeoutEnvVar        = "WORKER_TIMEOUT"
	defaultTimeout             = 5 * time.Minute
	bucketEnvVar               = "EXPORT_BUCKET"
	maxRecordsEnvVar           = "EXPORT_FILE_MAX_RECORDS"
	defaultMaxRecords          = 30_000
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

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

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		logger.Fatalf("unable to connect to database: %w", err)
	}
	defer db.Close(ctx)

	bsc := export.BatchServerConfig{}
	bsc.CreateTimeout = serverenv.ParseDuration(ctx, createBatchesTimeoutEnvVar, defaultTimeout)
	logger.Infof("Using create batches timeout %v (override with $%s)", bsc.CreateTimeout, createBatchesTimeoutEnvVar)

	bsc.WorkerTimeout = serverenv.ParseDuration(ctx, workerTimeoutEnvVar, defaultTimeout)
	logger.Infof("Using worker timeout %v (override with $%s)", bsc.WorkerTimeout, workerTimeoutEnvVar)

	if maxRecStr, ok := os.LookupEnv(maxRecordsEnvVar); !ok {
		logger.Infof("Using export file max size %d (override with $%s)", defaultMaxRecords, maxRecordsEnvVar)
		bsc.MaxRecords = defaultMaxRecords
	} else {
		if maxRec, err := strconv.Atoi(maxRecStr); err != nil {
			logger.Errorf("Failed to parse $%s value %q, using default %d", maxRecordsEnvVar, maxRecStr, defaultMaxRecords)
			bsc.MaxRecords = defaultMaxRecords
		} else {
			bsc.MaxRecords = maxRec
		}
	}
	if bucket, ok := os.LookupEnv(bucketEnvVar); !ok {
		logger.Fatalf("Required $%s is not specified.", bucketEnvVar)
	} else {
		bsc.Bucket = bucket
	}

	batchServer, err := export.NewBatchServer(db, bsc, env)
	if err != nil {
		logger.Fatalf("unable to create server: %v", err)
	}
	http.HandleFunc("/create-batches", batchServer.CreateBatchesHandler) // controller that creates work items
	http.HandleFunc("/do-work", batchServer.WorkerHandler)               // worker that executes work

	logger.Info("starting exposure export server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port), nil))
}
