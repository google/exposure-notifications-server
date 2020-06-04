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
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
)

// Compile-time check to assert this config matches requirements.
var _ setup.AuthorizedAppConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	AuthorizedApp authorizedapp.Config
	Database      database.Config
	SecretManager secrets.Config

	Port               string        `env:"PORT, default=8080"`
	MinRequestDuration time.Duration `env:"TARGET_REQUEST_DURATION, default=5s"`
	MaxKeysOnPublish   int           `env:"MAX_KEYS_ON_PUBLISH, default=15"`
	MaxIntervalAge     time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	TruncateWindow     time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// Flags for local development and testing.
	DebugAPIResponses       bool `env:"DEBUG_API_RESPONSES"`
	DebugReleaseSameDayKeys bool `env:"DEBUG_RELEASE_SAME_DAY_KEYS"`
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

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		AuthorizedApp: *authorizedapp.TestConfigDefaults(),
		Database:      *database.TestConfigDefaults(),
		SecretManager: *secrets.TestConfigDefaults(),

		Port:                "8080",
		MinRequestDuration:  5 * time.Second,
		MaxKeysOnPublish:    15,
		MaxIntervalAge:      360 * time.Hour,
		TruncateWindow:      1 * time.Hour,
		DebugAPIResponses:   false,
		DebugAllowRestOfDay: false,
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		AuthorizedApp: *authorizedapp.TestConfigValued(),
		Database:      *database.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),

		Port:                "5555",
		MinRequestDuration:  50 * time.Second,
		MaxKeysOnPublish:    150,
		MaxIntervalAge:      600 * time.Minute,
		TruncateWindow:      10 * time.Hour,
		DebugAPIResponses:   true,
		DebugAllowRestOfDay: true,
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":                        "5555",
		"TARGET_REQUEST_DURATION":     "50s",
		"MAX_KEYS_ON_PUBLISH":         "150",
		"MAX_INTERVAL_AGE_ON_PUBLISH": "600m",
		"TRUNCATE_WINDOW":             "10h",
		"DEBUG_API_RESPONSES":         "true",
		"DEBUG_ALLOW_REST_OF_DAY":     "true",
	}

	embedded := []map[string]string{
		authorizedapp.TestConfigValues(),
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
		AuthorizedApp: *authorizedapp.TestConfigOverridden(),
		Database:      *database.TestConfigOverridden(),
		SecretManager: *secrets.TestConfigOverridden(),

		Port:                "4444",
		MinRequestDuration:  20 * time.Second,
		MaxKeysOnPublish:    250,
		MaxIntervalAge:      200 * time.Minute,
		TruncateWindow:      20 * time.Hour,
		DebugAPIResponses:   true,
		DebugAllowRestOfDay: true,
	}
}
