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
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/googlepartners/exposure-notifications/pkg/api"
	"github.com/googlepartners/exposure-notifications/pkg/database"
	"github.com/googlepartners/exposure-notifications/pkg/logging"
	"github.com/googlepartners/exposure-notifications/pkg/serverenv"
)

var (
	timeoutEnvVar  = "PULL_TIMEOUT"
	defaultTimeout = 5 * time.Minute
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

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

	db, err := database.NewFromEnv(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	http.Handle("/", api.NewFederationPullHandler(db, timeout))
	logger.Info("starting federation puller")
	env := serverenv.New(ctx)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port()), nil))
}
