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

// Package migrate handles the configuration and execution of database migrations
package migrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

// New makes a new, configured Migration.
func New(config *Config, env *serverenv.ServerEnv) (*Migration, error) {
	// Validate config.
	if env.Database() == nil {
		return nil, fmt.Errorf("migrate.New requires Database present in the ServerEnv")
	}

	return &Migration{
		config: config,
		env:    env,
	}, nil
}

// Migration wraps the configuration required to execute a migration against the database.
type Migration struct {
	config *Config
	env    *serverenv.ServerEnv
}

// Run executes the provided command against the migrate binary
// using the configured database and migrations path
func (m *Migration) Run(ctx context.Context) error {
	args := []string{
		"-database",
		database.ConnectionString(m.config.DatabaseConfig()),
		"-path",
		m.config.Migrations,
		m.config.MigrateCommand,
	}
	cmd := exec.Command(m.config.MigrateBinary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
