// Copyright 2020 the Exposure Notifications Server authors
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

// Package generate contains HTTP handler for triggering data generation into the databae.
package generate

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var (
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.SecretManagerConfigProvider         = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
	_ model.TransformerConfig                   = (*Config)(nil)
)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	Database              database.Config
	SecretManager         secrets.Config
	ObservabilityExporter observability.Config

	Port                         string        `env:"PORT, default=8080"`
	NumExposures                 int           `env:"NUM_EXPOSURES_GENERATED, default=10"`
	KeysPerExposure              int           `env:"KEYS_PER_EXPOSURE, default=14"`
	MaxKeysOnPublish             uint          `env:"MAX_KEYS_ON_PUBLISH, default=15"`
	MaxSameStartIntervalKeys     uint          `env:"MAX_SAME_START_INTERVAL_KEYS, default=2"`
	SimulateSameDayRelease       bool          `env:"SIMULATE_SAME_DAY_RELEASE, default=false"`
	MaxIntervalAge               time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	MaxMagnitudeSymptomOnsetDays uint          `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`
	MaxSymptomOnsetReportDays    uint          `env:"MAX_VALID_SYMPTOM_ONSET_REPORT_DAYS, default=28"`
	CreatedAtTruncateWindow      time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	DefaultRegion                string        `env:"DEFAULT_REGION, default=US"`
	ChanceOfKeyRevision          int           `env:"CHANCE_OF_KEY_REVISION, default=30"` // 0-100 are valid values.
	ChanceOfTraveler             int           `env:"CHANCE_OF_TRAVELER, default=20"`     // 0-100 are valid values
	KeyRevisionDelay             time.Duration `env:"KEY_REVISION_DELAY, default=2h"`     // key revision will be forward dates this amount.
	SymptomOnsetDaysAgo          uint          `env:"DEFAULT_SYMPTOM_ONSET_DAYS_AGO, default=4"`
	ForceConfirmed               bool          `env:"FORCE_CONFIRMED, default=false"` // force report type to be confirmed for all exposures
}

func (c *Config) MaxExposureKeys() uint {
	return c.MaxKeysOnPublish
}

func (c *Config) MaxSameDayKeys() uint {
	return c.MaxSameStartIntervalKeys
}

func (c *Config) MaxIntervalStartAge() time.Duration {
	return c.MaxIntervalAge
}

func (c *Config) TruncateWindow() time.Duration {
	return c.CreatedAtTruncateWindow
}

func (c *Config) MaxSymptomOnsetDays() uint {
	return c.MaxMagnitudeSymptomOnsetDays
}

func (c *Config) MaxValidSymptomOnsetReportDays() uint {
	return c.MaxSymptomOnsetReportDays
}

func (c *Config) DefaultSymptomOnsetDaysAgo() uint {
	return c.SymptomOnsetDaysAgo
}

func (c *Config) DebugReleaseSameDayKeys() bool {
	return false
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
