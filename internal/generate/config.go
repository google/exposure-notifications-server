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

// Package generate contains HTTP handler for triggering data generation into the databae.
package generate

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	Database      database.Config
	SecretManager secrets.Config

	Port             string        `env:"PORT, default=8080"`
	NumExposures     int           `env:"NUM_EXPOSURES_GENERATED, default=10"`
	KeysPerExposure  int           `env:"KEYS_PER_EXPOSURE, default=14"`
	MaxKeysOnPublish int           `env:"MAX_KEYS_ON_PUBLISH, default=15"`
	MaxIntervalAge   time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	TruncateWindow   time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	DefaultRegion    string        `env:"DEFAULT_REGOIN, default=US"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		Database:      *database.TestConfigDefaults(),
		SecretManager: *secrets.TestConfigDefaults(),

		Port:             "8080",
		NumExposures:     10,
		KeysPerExposure:  14,
		MaxKeysOnPublish: 15,
		MaxIntervalAge:   360 * time.Hour,
		TruncateWindow:   1 * time.Hour,
		DefaultRegion:    "US",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Database:      *database.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),

		Port:             "5555",
		NumExposures:     20,
		KeysPerExposure:  24,
		MaxKeysOnPublish: 25,
		MaxIntervalAge:   260 * time.Hour,
		TruncateWindow:   2 * time.Hour,
		DefaultRegion:    "CA",
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":                        "5555",
		"NUM_EXPOSURES_GENERATED":     "20",
		"KEYS_PER_EXPOSURE":           "24",
		"MAX_KEYS_ON_PUBLISH":         "25",
		"MAX_INTERVAL_AGE_ON_PUBLISH": "260h",
		"TRUNCATE_WINDOW":             "2h",
		"DEFAULT_REGOIN":              "CA",
	}

	embedded := []map[string]string{
		database.TestConfigValues(),
		secrets.TestConfigValues(),
	}
	for _, c := range embedded {
		for k, v := range c {
			m[k] = v
		}
	}

	return m
}

// TestConfigOverridden returns a configuration with non-default values set. It
// should only be used for testing.
func TestConfigOverridden() *Config {
	return &Config{
		Database:      *database.TestConfigOverridden(),
		SecretManager: *secrets.TestConfigOverridden(),

		Port:             "4444",
		NumExposures:     30,
		KeysPerExposure:  34,
		MaxKeysOnPublish: 35,
		MaxIntervalAge:   160 * time.Hour,
		TruncateWindow:   3 * time.Hour,
		DefaultRegion:    "LP",
	}
}
