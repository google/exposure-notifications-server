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

	"go.opencensus.io/plugin/ochttp"

	"github.com/google/exposure-notifications-server/internal/handlers"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	var config publish.Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		logger.Fatalf("setup.Setup: %v", err)
	}
	defer closer()

	handler, err := publish.NewHandler(ctx, &config, env)
	if err != nil {
		logger.Fatalf("unable to create publish handler: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", handlers.WithMinimumLatency(config.MinRequestDuration, handler))
	logger.Infof("starting exposure server on :%s", config.Port)
	instrumentedHandler := &ochttp.Handler{Handler: mux}
	log.Fatal(http.ListenAndServe(":"+config.Port, instrumentedHandler))
}
