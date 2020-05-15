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

// Package main runs all the server components at different URL paths.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/api/export"
	"github.com/google/exposure-notifications-server/internal/api/federationin"
	"github.com/google/exposure-notifications-server/internal/api/handlers"
	"github.com/google/exposure-notifications-server/internal/api/publish"
	cleanup "github.com/google/exposure-notifications-server/internal/cleanup/api"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/dbapiconfig"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/setup"
)

type MonoConfig struct {
	Port string `envconfig:"PORT" default:"8080"`

	APIConfigOpts *dbapiconfig.ConfigOpts
	Cleanup       *cleanup.Config
	Export        *export.Config
	Publish       *publish.Config
	Database      *database.Config
	FederationIn  *federationin.Config
}

func (c *MonoConfig) DB() *database.Config         { return c.Database }
func (c *MonoConfig) KeyManager() bool             { return true }
func (c *MonoConfig) BlobStorage() bool            { return true }
func (c *MonoConfig) API() *dbapiconfig.ConfigOpts { return c.APIConfigOpts }

func main() {
	ctx := context.Background()
	logger := logging.FromContext(ctx)

	if err := realMain(ctx); err != nil {
		logger.Fatal(err)
	}
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config MonoConfig
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer closer()

	// Cleanup export
	cleanupExport, err := cleanup.NewExportHandler(config.Cleanup, env)
	if err != nil {
		return fmt.Errorf("cleanup.NewExportHandler: %w", err)
	}
	http.Handle("/cleanup-export", cleanupExport)

	// Cleanup exposure
	cleanupExposure, err := cleanup.NewExposureHandler(config.Cleanup, env)
	if err != nil {
		return fmt.Errorf("cleanup.NewExposureHandler: %w", err)
	}
	http.Handle("/cleanup-exposure", cleanupExposure)

	// Export
	exportServer, err := export.NewServer(config.Export, env)
	if err != nil {
		return fmt.Errorf("export.NewServer: %w", err)
	}
	http.HandleFunc("/export/create-batches", exportServer.CreateBatchesHandler)
	http.HandleFunc("/export/do-work", exportServer.WorkerHandler)

	// Federation in
	http.Handle("/federation-in", federationin.NewHandler(env, config.FederationIn))

	// Federation out
	// TODO: this is a grpc listener and requires a lot of setup.

	// Publish
	publishServer, err := publish.NewHandler(ctx, config.Publish, env)
	if err != nil {
		return fmt.Errorf("publish.NewHandler: %w", err)
	}
	http.HandleFunc("/publish", handlers.WithMinimumLatency(config.Publish.MinRequestDuration, publishServer))

	logger.Infof("monolith running at :%s", config.Port)
	return http.ListenAndServe(":"+config.Port, nil)
}
