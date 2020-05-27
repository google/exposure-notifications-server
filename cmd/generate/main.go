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

// Deployable server to generate fake exposure keys on demand.
// THIS IS NOT INTENED TO RUN IN PRODUCTION.
package main

import (
	"context"
	"log"
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/google/exposure-notifications-server/internal/generate"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	var config generate.Config
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		logger.Fatalf("setup.Setup: %v", err)
	}
	defer closer()

	handler, err := generate.NewHandler(ctx, &config, env)
	if err != nil {
		logger.Fatalf("unable to create generate handler: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	logger.Infof("starting generate server on :%s", config.Port)
	instrumentedHandler := &ochttp.Handler{Handler: mux}
	log.Fatal(http.ListenAndServe(":"+config.Port, instrumentedHandler))
}
