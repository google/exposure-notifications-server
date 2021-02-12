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
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

var (
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
	_ setup.SecretManagerConfigProvider         = (*Config)(nil)
)

type Config struct {
	Database              database.Config
	ObservabilityExporter observability.Config
	SecretManager         secrets.Config

	Port string `env:"PORT, default=8080"`

	IndexFileDownloadTimeout  time.Duration `env:"INDEX_FILE_DOWNLOAD_TIMEOUT, default=30s"`
	ExportFileDownloadTimeout time.Duration `env:"EXPORT_FILE_DOWNLOAD_TIMEOUT, default=2m"`

	// For importing files that may have missed setting v1.5+ fields.
	BackfillReportType          string `env:"BACKFILL_REPORT_TYPE, default=confirmed"`
	BackfillDaysSinceOnset      bool   `env:"BACKFILL_DAYS_SINCE_ONSET, default=true"`
	BackfillDaysSinceOnsetValue int    `env:"BACKFILL_DAYS_SINCE_ONSET_VALUE, default=10"`

	MaxInsertBatchSize           int           `env:"MAX_INSERT_BATCH_SIZE, default=100"`
	MaxIntervalAge               time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	MaxMagnitudeSymptomOnsetDays uint          `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`
	CreatedAtTruncateWindow      time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	// Each import config is locked while file are being imported. This is to prevent starvation in
	// the event that there are multiple exportimport configurations. If a worker fails
	// during processing, this defines the lock timeout.
	ImportLockTime time.Duration `env:"IMPORT_LOCK_TIME, default=13m"`
	// Maximum amount of time that an import worker should be allowed to run. This should be set
	// lower that infrastructure level request timeouts and lower than the lock time.
	MaxRuntime time.Duration `env:"MAX_RUNTIME, default=12m"`
	// Each exposure is inserted with the app_package_name / healthAuthorityID that it was published with
	// Use this string to signal that a key came from the export-importer job.
	ImportAPKName string `env:"IMPORT_APP_PACKAGE_NAME, default=exportimport"`
	// ImportRetryRate is the rate at which files that encounter an error while
	// importing are retried.
	ImportRetryRate time.Duration `env:"IMPORT_RETRY_RATE, default=6h"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}
