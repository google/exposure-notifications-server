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

package cleanup

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// Compile-time check to assert this config matches requirements.
var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the cleanup components.
type Config struct {
	Storage       *storage.Config
	Database      *database.Config
	SecretManager *secrets.Config

	Port    string        `envconfig:"PORT" default:"8080"`
	Timeout time.Duration `envconfig:"CLEANUP_TIMEOUT" default:"10m"`
	TTL     time.Duration `envconfig:"CLEANUP_TTL" default:"336h"`
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return c.Storage
}

func (c *Config) DatabaseConfig() *database.Config {
	return c.Database
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return c.SecretManager
}
