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

// This package is the service rotates revision token encryption keys; it is intended to be invoked over HTTP by Cloud Scheduler.
package main

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/keyrotation"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/setup"
	_ "github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/sethvargo/go-signalcontext"
)

func main() {
	ctx, done := signalcontext.OnInterrupt()
	logger := logging.FromContext(ctx)

	err := realMain(ctx)
	done()

	if err != nil {
		logger.Fatal(err)
	}
	logger.Info("successful shutdown")
}

func realMain(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	var config keyrotation.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	rotationServer, err := keyrotation.NewServer(&config, env)
	if err != nil {
		return fmt.Errorf("keyrotation.NewServer: %w", err)
	}

	srv, err := server.New(config.Port)
	if err != nil {
		return fmt.Errorf("server.New: %w", err)
	}
	logger.Info("listening on: ", config.Port)

	return srv.ServeHTTPHandler(ctx, rotationServer.Routes(ctx))
}
