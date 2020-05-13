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

// This package is the service that deletes old exposure keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/api/cleanup"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/envconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/secrets"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	// It is possible to install a different secret management system here that conforms to secrets.SecretManager{}
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}

	var config cleanup.Config
	if err := envconfig.Process(ctx, &config, sm); err != nil {
		logger.Fatalf("error loading environment variables: %v", err)
	}

	db, err := database.NewFromEnv(ctx, config.Database)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	http.Handle("/", cleanup.NewExposureHandler(db, &config))
	logger.Infof("starting cleanup server on :%s", config.Port)
	log.Fatal(http.ListenAndServe(":"+config.Port, nil))
}
