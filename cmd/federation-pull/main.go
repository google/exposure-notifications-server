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

// This package is the service that pulls federation results from federation partners; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"cambio/pkg/api"
	"cambio/pkg/database"
	"cambio/pkg/logging"
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

var (
	portEnvVar     = "PORT"
	defaultPort    = "8080"
	timeoutEnvVar  = "PULL_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	port := os.Getenv(portEnvVar)
	if port == "" {
		port = defaultPort
	}
	logger.Infof("Using port %s (override with $%s)", port, portEnvVar)

	timeout := defaultTimeout
	if timeoutStr := os.Getenv(timeoutEnvVar); timeoutStr != "" {
		var err error
		timeout, err = time.ParseDuration(timeoutStr)
		if err != nil {
			logger.Warnf("Failed to parse $%s value %q, using default.", timeoutEnvVar, timeoutStr)
			timeout = defaultTimeout
		}
	}
	logger.Infof("Using fetch timeout %v (override with $%s)", timeout, timeoutEnvVar)

	cleanup, err := database.Initialize(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer cleanup(ctx)

	router := mux.NewRouter()
	router.Handle("/", api.FederationPullHandler{Timeout: timeout})
	logger.Info("starting federation puller")
	log.Fatal(http.ListenAndServe(":"+port, router))
}
