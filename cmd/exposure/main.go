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
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/setup"
	_ "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLogger(true)
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config publish.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	mux := http.NewServeMux()
	handler, err := publish.NewHandler(ctx, &config, env)
	if err != nil {
		return fmt.Errorf("publish.NewHandler: %w", err)
	}
	// Serving of v1alpha1 is on by default, but can be disabled through env var.
	if config.EnableV1Alpha1API {
		mux.Handle("/", handler.HandleV1Alpha1())
	}
	mux.Handle("/v1/publish", handler.Handle())

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("server.New: %w", err)
	}
	logger.Infof("listening on :%s", config.Port)

	return srv.ServeHTTPHandler(ctx, mux)
}
