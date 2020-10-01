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

package exportimport

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/observability"
)

var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)

type Config struct {
	Database              database.Config
	ObservabilityExporter observability.Config

	Port string `env:"PORT, default=8080"`

	MaxInsertBatchSize           int           `env:"MAX_INSERT_BATCH_SIZE, default=100"`
	MaxIntervalAge               time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	MaxMagnitudeSymptomOnsetDays uint          `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`
	CreatedAtTruncateWindow      time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	ImportLockTime               time.Duration `env:"IMPORT_LOCK_TIME, default=13m"`
	MaxRuntime                   time.Duration `env:"MAX_RUNTIME, default=12m"`
	ImportAPKName                string        `env:"IMPORT_APP_PACKAGE_NAME, default=exportimport"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}
