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

// This package is used to apply database migrations
package main

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/interrupt"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/migrate"
	_ "github.com/google/exposure-notifications-server/internal/observability"
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

	var config migrate.Config
	env, err := setup.Setup(ctx, &config)
	if err != nil {
		return fmt.Errorf("setup.Setup: %w", err)
	}
	defer env.Close(ctx)

	m, err := migrate.New(&config, env)
	if err != nil {
		return fmt.Errorf("migrate.New: %w", err)
	}

	logger.Infof("beginning migration")

	if err := m.Run(ctx); err != nil {
		return fmt.Errorf("migrate.Run: %w", err)
	}

	logger.Infof("migration completed")

	return nil
}
