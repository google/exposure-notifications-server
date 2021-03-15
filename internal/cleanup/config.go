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

	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var (
	_ setup.BlobstoreConfigProvider             = (*Config)(nil)
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.SecretManagerConfigProvider         = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
)

// Config represents the configuration and associated environment variables for
// the cleanup components.
type Config struct {
	Database              database.Config
	SecretManager         secrets.Config
	Storage               storage.Config
	ObservabilityExporter observability.Config

	Port    string        `env:"PORT, default=8080"`
	Timeout time.Duration `env:"CLEANUP_TIMEOUT, default=10m"`
	TTL     time.Duration `env:"CLEANUP_TTL, default=336h"`

	DebugOverrideCleanupMinDuration bool `env:"DEBUG_OVERRIDE_CLEANUP_MIN_DURATION, default=false"`
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return &c.Storage
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}
