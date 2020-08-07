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

package keyrotation

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/revision"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the key rotation components.
type Config struct {
	Database              database.Config
	SecretManager         secrets.Config
	ObservabilityExporter observability.Config
	RevisionToken         revision.Config
	KeyManager            keys.Config

	Port string `env:"PORT, default=8080"`

	// NewKeyPeriod is the duration after which we will rotate encryption keys. By default we
	// generate a new key every two weeks.
	NewKeyPeriod time.Duration `env:"NEW_KEY_PERIOD, default=168h"`

	// DeleteOldKeyPeriod is the duration after which it is safe to delete old keys.
	// We delete old data after two weeks after which it should be safe to also delete
	// the associated key - we default to 15d to buffer for potential timezones issues.
	DeleteOldKeyPeriod time.Duration `env:"DELETE_OLD_KEY_PERIOD, default=360h"`
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

func (c *Config) KeyManagerConfig() *keys.Config {
	return &c.KeyManager
}
