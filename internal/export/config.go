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

// Package export defines the handlers for managing exposure key exporting.
package export

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the export components.
type Config struct {
	Database              database.Config
	KeyManager            keys.Config
	SecretManager         secrets.Config
	Storage               storage.Config
	ObservabilityExporter observability.Config

	Port           string        `env:"PORT, default=8080"`
	CreateTimeout  time.Duration `env:"CREATE_BATCHES_TIMEOUT, default=5m"`
	WorkerTimeout  time.Duration `env:"WORKER_TIMEOUT, default=5m"`
	MinRecords     int           `env:"EXPORT_FILE_MIN_RECORDS, default=1000"`
	PaddingRange   int           `env:"EXPORT_FILE_PADDING_RANGE, default=100"`
	MaxRecords     int           `env:"EXPORT_FILE_MAX_RECORDS, default=30000"`
	TruncateWindow time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	MinWindowAge   time.Duration `env:"MIN_WINDOW_AGE, default=2h"`
	TTL            time.Duration `env:"CLEANUP_TTL, default=336h"`
}

func (c *Config) BlobstoreConfig() *storage.Config {
	return &c.Storage
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) KeyManagerConfig() *keys.Config {
	return &c.KeyManager
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}
