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

// Package publish defines the exposure keys publishing API.
package publish

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/observability"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/verification"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var _ setup.AuthorizedAppConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
var _ model.TransformerConfig = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	AuthorizedApp         authorizedapp.Config
	Database              database.Config
	SecretManager         secrets.Config
	Verification          verification.Config
	ObservabilityExporter observability.Config

	Port             string `env:"PORT, default=8080"`
	MaxKeysOnPublish uint   `env:"MAX_KEYS_ON_PUBLISH, default=30"`
	// Provides compatibility w/ 1.5 release.
	MaxSameStartIntervalKeys     uint          `env:"MAX_SAME_START_INTERVAL_KEYS, default=3"`
	MaxIntervalAge               time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	MaxMagnitudeSymptomOnsetDays uint          `env:"MAX_SYMPTOM_ONSET_DAYS, default=21"`
	CreatedAtTruncateWindow      time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// Flags for local development and testing. This will cause still valid keys
	// to not be embargoed.
	// Normally "still valid" keys can be accepted, but are embargoed.
	ReleaseSameDayKeys bool `env:"DEBUG_RELEASE_SAME_DAY_KEYS"`
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

func (c *Config) DebugReleaseSameDayKeys() bool {
	return c.ReleaseSameDayKeys
}

func (c *Config) AuthorizedAppConfig() *authorizedapp.Config {
	return &c.AuthorizedApp
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
