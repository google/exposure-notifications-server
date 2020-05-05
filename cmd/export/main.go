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
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	createBatchesTimeoutEnvVar = "CREATE_BATCHES_TIMEOUT"
	defaultTimeout             = 5 * time.Minute
	bucketEnvVar               = "EXPORT_BUCKET"
	tmpBucketEnvVar            = "TMP_EXPORT_BUCKET"
	maxRecordsEnvVar           = "EXPORT_FILE_MAX_RECORDS"
	defaultMaxRecords          = 30_000
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	env, err := serverenv.New(ctx).WithSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	bsc := export.BatchServerConfig{}
	bsc.CreateTimeout = serverenv.ParseDuration(ctx, createBatchesTimeoutEnvVar, defaultTimeout)
	logger.Infof("Using create batches timeout %v (override with $%s)", bsc.CreateTimeout, createBatchesTimeoutEnvVar)

	bsc.MaxRecords, err = strconv.Atoi(os.Getenv(maxRecordsEnvVar))
	if err != nil {
		logger.Infof("Failed to parse export batch size env EnvVar: %v, %v", maxRecordsEnvVar, err)
		bsc.MaxRecords = defaultMaxRecords
	}
	bsc.TmpBucket = os.Getenv(tmpBucketEnvVar)
	bsc.Bucket = os.Getenv(bucketEnvVar)

	// TODO(guray): remove or gate the /test handler
	http.Handle("/test", export.NewTestExportHandler(db))

	batchServer := export.NewBatchServer(db, bsc)
	http.HandleFunc("/create-batches", batchServer.CreateBatchesHandler) // controller that creates work items
	http.HandleFunc("/create-files", batchServer.CreateFilesHandler)     // worker that executes work

	logger.Info("starting exposure export server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port()), nil))
}
