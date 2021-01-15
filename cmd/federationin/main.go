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

// This package is the service that pulls federation results from other federation severs.
// It is intended to be invoked over HTTP on a schedule.
package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/buildinfo"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/logging"
	_ "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()

	logger := logging.NewLoggerFromEnv().
		With("build_id", buildinfo.KeyServer.ID()).
		With("build_tag", buildinfo.KeyServer.Tag())
	ctx = logging.WithLogger(ctx, logger)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
}
func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config federationin.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	handler := federationin.NewHandler(env, &config)

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.Handle("/health", server.HandleHealthz(ctx))

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("server.New: %w", err)
	}
	logger.Infof("listening on :%s", config.Port)

	return srv.ServeHTTPHandler(ctx, mux)
}
