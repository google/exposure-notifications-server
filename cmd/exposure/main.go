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
	"net/http"

	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()
	defer done()

	if err := realMain(ctx); err != nil {
		logger := logging.FromContext(ctx)
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config publish.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	if !(config.EnableV1API || config.EnableV1Alpha1API) {
		return fmt.Errorf("no APIs enabled, must set one or more of ENABLE_V1_API, ENABLE_V1ALPHA1_API")
	}

	mux := http.NewServeMux()
	if config.EnableV1Alpha1API {
		handler, err := publish.NewV1Alpha1Handler(ctx, &config, env)
		if err != nil {
			return fmt.Errorf("publish.NewHandler: %w", err)
		}
		mux.Handle("/", handler)
	}
	if config.EnableV1API {
		handler, err := publish.NewV1Handler(ctx, &config, env)
		if err != nil {
			return fmt.Errorf("publish.NewHandler: %w", err)
		}
		mux.Handle("/v1/publish", handler)
	}

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("server.New: %w", err)
	}
	logger.Infof("listening on :%s", config.Port)

	return srv.ServeHTTPHandler(ctx, mux)
}
