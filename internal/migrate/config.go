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

package migrate

import (
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config represents the configuration for the migrate components.
type Config struct {
	Database      database.Config
	SecretManager secrets.Config

	// MigrateBinary is the location of the go-migrate binary.
	MigrateBinary string `env:"MIGRATE_BINARY, default=/bin/gomigrate"`
	// MigrateCommand is the command run against the migrate binary.
	MigrateCommand string `env:"MIGRATE_COMMAND, default=up"`
	// Migrations is the path to the directory containing the migration files.
	Migrations string `env:"MIGRATIONS, default=migrations"`
}

// DatabaseConfig returns the configuration for the database.
func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

// SecretManagerConfig returns the configuration for the secrets manager.
func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}
