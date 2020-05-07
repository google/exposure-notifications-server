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
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/api/handlers"
	"github.com/google/exposure-notifications-server/internal/api/publish"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

const (
	minPublishDurationEnv     = "MIN_PUBLISH_DURATION"
	defaultMinPublishDiration = 5 * time.Second
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	// It is possible to install a different secret management system here that conforms to secrets.SecretManager{}
	sm, err := secrets.NewGCPSecretManager(ctx)
	if err != nil {
		logger.Fatalf("unable to connect to secret manager: %v", err)
	}
	env := serverenv.New(ctx,
		serverenv.WithSecretManager(sm),
		serverenv.WithMetricsExporter(metrics.NewLogsBasedFromContext))

	db, err := database.NewFromEnv(ctx, env)
	if err != nil {
		logger.Fatalf("unable to connect to database: %v", err)
	}
	defer db.Close(ctx)

	cfg := config.New(db)

	minLatency := serverenv.ParseDuration(ctx, minPublishDurationEnv, defaultMinPublishDiration)
	logger.Infof("Request minimum latency is: %v", minLatency.String())

	http.Handle("/", handlers.WithMinimumLatency(minLatency, publish.NewHandler(ctx, db, cfg, env)))
	logger.Info("starting exposure server")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", env.Port()), nil))
}
