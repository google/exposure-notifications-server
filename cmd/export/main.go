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
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/logging"
	_ "github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/setup"
)

func main() {
	ctx := context.Background()
	config := export.Config{}
	if err := realMain(ctx, &config); err != nil {
		logger := logging.FromContext(context.Background())
		logger.Fatalf("export: %v", err)
	}
}

func realMain(ctx context.Context, config *export.Config) error {
	logger := logging.FromContext(ctx)

	env, closer, err := setup.Setup(ctx, config)
	if err != nil {
		return fmt.Errorf("setup: %w", err)
	}
	defer closer()

	batchServer, err := export.NewServer(config, env)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/create-batches", batchServer.CreateBatchesHandler)
	mux.HandleFunc("/do-work", batchServer.WorkerHandler)

	server := &http.Server{
		Addr: ":" + config.Port,
		Handler: &ochttp.Handler{
			Handler: mux,
		},
	}

	go func() {
		logger.Infof("starting export server on :%s", config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("failed to run server: %v", err)
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}

	return nil
}
