// Copyright 2021 Google LLC
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

package backup

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/observability"
)

// Compile-time check to assert this config matches requirements.
var (
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
)

// Config represents the configuration and associated environment variables for
// the cleanup components.
type Config struct {
	Database              database.Config
	ObservabilityExporter observability.Config

	Port string `env:"PORT, default=8080"`

	// MinTTL is the minimum amount of time that must elapse between attempting
	// backups. This is used to control whether the pull is actually attempted at
	// the controller layer, independent of the data layer. In effect, it rate
	// limits the number of requests.
	MinTTL time.Duration `env:"BACKUP_MIN_PERIOD, default=4h"`

	// Timeout is the maximum amount of time to wait for a backup operation to
	// complete.
	Timeout time.Duration `env:"BACKUP_TIMEOUT, default=10m"`

	// Bucket is the name of the Cloud Storage bucket where backups should be
	// stored.
	Bucket string `env:"BACKUP_BUCKET, required"`

	// DatabaseInstanceURL is the full self-link of the URL to the SQL instance.
	DatabaseInstanceURL string `env:"BACKUP_DATABASE_INSTANCE_URL, required"`

	// DatabaseName is the name of the database to backup.
	DatabaseName string `env:"BACKUP_DATABASE_NAME, required"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}
