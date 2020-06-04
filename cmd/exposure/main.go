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
	"time"

	"github.com/google/exposure-notifications-server/internal/handlers"
	"github.com/google/exposure-notifications-server/internal/interrupt"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/server"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx, done := interrupt.Context()
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

	handler, err := publish.NewHandler(ctx, &config, env)
	if err != nil {
		return fmt.Errorf("publish.NewHandler: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handlers.WithMinimumLatency(config.MinRequestDuration, handler))

	server := server.New(config.Port, mux)
	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("server.Start: %w", err)
	}
	logger.Infof("listening on :%s", config.Port)

	// Wait for cancel or interrupt
	<-ctx.Done()

	// Shutdown
	logger.Info("received shutdown")
	shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()

	if err := server.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("server.Stop: %w", err)
	}

	logger.Info("shutdown complete")
	return nil
}
