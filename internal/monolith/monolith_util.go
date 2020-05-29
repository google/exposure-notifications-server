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
package monolith

import (
	"context"
	"fmt"
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/cleanup"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/handlers"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"

	// Enable observability with distributed tracing and metrics.
	_ "github.com/google/exposure-notifications-server/internal/observability"
)

var _ setup.AuthorizedAppConfigProvider = (*MonoConfig)(nil)
var _ setup.BlobstoreConfigProvider = (*MonoConfig)(nil)
var _ setup.DatabaseConfigProvider = (*MonoConfig)(nil)
var _ setup.KeyManagerConfigProvider = (*MonoConfig)(nil)
var _ setup.SecretManagerConfigProvider = (*MonoConfig)(nil)

type MonoConfig struct {
	AuthorizedApp *authorizedapp.Config
	Storage       *storage.Config
	Cleanup       *cleanup.Config
	Database      *database.Config
	Export        *export.Config
	FederationIn  *federationin.Config
	KeyManager    *signing.Config
	Publish       *publish.Config
	SecretManager *secrets.Config

	Port string `envconfig:"PORT" default:"8080"`
}

func (c *MonoConfig) AuthorizedAppConfig() *authorizedapp.Config {
	return c.AuthorizedApp
}

func (c *MonoConfig) BlobstoreConfig() *storage.Config {
	return c.Storage
}

func (c *MonoConfig) DatabaseConfig() *database.Config {
	return c.Database
}

func (c *MonoConfig) KeyManagerConfig() *signing.Config {
	return c.KeyManager
}

func (c *MonoConfig) SecretManagerConfig() *secrets.Config {
	return c.SecretManager
}

func RunServer(ctx context.Context) (*MonoConfig, error) {
	logger := logging.FromContext(ctx)

	var config MonoConfig
	env, closer, err := setup.Setup(ctx, &config)
	if err != nil {
		return nil, fmt.Errorf("setup.Setup: %w", err)
	}
	defer closer()

	mux := http.NewServeMux()

	// Cleanup export
	cleanupExport, err := cleanup.NewExportHandler(config.Cleanup, env)
	if err != nil {
		return nil, fmt.Errorf("cleanup.NewExportHandler: %w", err)
	}
	mux.Handle("/cleanup-export", cleanupExport)

	// Cleanup exposure
	cleanupExposure, err := cleanup.NewExposureHandler(config.Cleanup, env)
	if err != nil {
		return nil, fmt.Errorf("cleanup.NewExposureHandler: %w", err)
	}
	mux.Handle("/cleanup-exposure", cleanupExposure)

	// Export
	exportServer, err := export.NewServer(config.Export, env)
	if err != nil {
		return nil, fmt.Errorf("export.NewServer: %w", err)
	}
	mux.HandleFunc("/export/create-batches", exportServer.CreateBatchesHandler)
	mux.HandleFunc("/export/do-work", exportServer.WorkerHandler)

	// Federation in
	mux.Handle("/federation-in", federationin.NewHandler(env, config.FederationIn))

	// Federation out
	// TODO: this is a grpc listener and requires a lot of setup.

	// Publish
	publishServer, err := publish.NewHandler(ctx, config.Publish, env)
	if err != nil {
		return nil, fmt.Errorf("publish.NewHandler: %w", err)
	}
	mux.HandleFunc("/publish", handlers.WithMinimumLatency(config.Publish.MinRequestDuration, publishServer))

	instrumentedHandler := &ochttp.Handler{Handler: mux}

	logger.Infof("monolith running at :%s", config.Port)
	return &config, http.ListenAndServe(":"+config.Port, instrumentedHandler)
}
