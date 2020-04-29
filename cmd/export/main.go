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
	"time"

	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"cambio/pkg/serverenv"
)

const (
	createBatchesTimeoutEnvVar = "CREATE_BATCHES_TIMEOUT"
	defaultTimeout             = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	db, err := database.NewFromEnv(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	createBatchesTimeout := defaultTimeout
	if timeoutStr := os.Getenv(createBatchesTimeoutEnvVar); timeoutStr != "" {
		var err error
		createBatchesTimeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warnf("Failed to parse $%s value %q, using default.", createBatchesTimeoutEnvVar, timeoutStr)
			createBatchesTimeout = defaultTimeout
		}
	}
	logger.Infof("Using create batches timeout %v (override with $%s)", createBatchesTimeout, createBatchesTimeoutEnvVar)

	// TODO(guray): remove or gate the /test handler
	http.Handle("/test", api.NewTestExportHandler(db))

	batchServer := api.NewBatchServer(db, createBatchesTimeout)
	http.HandleFunc("/create-batches", batchServer.CreateBatchesHandler)
	http.HandleFunc("/lease-batch", batchServer.LeaseBatchHandler)
	http.HandleFunc("/complete-batch", batchServer.CompleteBatchHandler)

	env := serverenv.New(ctx)
	logger.Info("starting infection export server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port()), nil))
}
