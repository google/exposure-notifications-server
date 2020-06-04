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

package export

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// Compile-time check to assert this config matches requirements.
var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the export components.
type Config struct {
	Database      database.Config
	KeyManager    signing.Config
	SecretManager secrets.Config
	Storage       storage.Config

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

func (c *Config) KeyManagerConfig() *signing.Config {
	return &c.KeyManager
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		Database:      *database.TestConfigDefaults(),
		KeyManager:    *signing.TestConfigDefaults(),
		SecretManager: *secrets.TestConfigDefaults(),
		Storage:       *storage.TestConfigDefaults(),

		Port:           "8080",
		CreateTimeout:  5 * time.Minute,
		WorkerTimeout:  5 * time.Minute,
		MinRecords:     1000,
		PaddingRange:   100,
		MaxRecords:     30000,
		TruncateWindow: 1 * time.Hour,
		MinWindowAge:   2 * time.Hour,
		TTL:            336 * time.Hour,
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Database:      *database.TestConfigValued(),
		KeyManager:    *signing.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),
		Storage:       *storage.TestConfigValued(),

		Port:           "5555",
		CreateTimeout:  15 * time.Minute,
		WorkerTimeout:  15 * time.Minute,
		MinRecords:     2000,
		PaddingRange:   200,
		MaxRecords:     40000,
		TruncateWindow: 2 * time.Hour,
		MinWindowAge:   3 * time.Hour,
		TTL:            447 * time.Hour,
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":                      "5555",
		"CREATE_BATCHES_TIMEOUT":    "15m",
		"WORKER_TIMEOUT":            "15m",
		"EXPORT_FILE_MIN_RECORDS":   "2000",
		"EXPORT_FILE_PADDING_RANGE": "200",
		"EXPORT_FILE_MAX_RECORDS":   "40000",
		"TRUNCATE_WINDOW":           "2h",
		"MIN_WINDOW_AGE":            "3h",
		"CLEANUP_TTL":               "447h",
	}

	embedded := []map[string]string{
		database.TestConfigValues(),
		signing.TestConfigValues(),
		secrets.TestConfigValues(),
		storage.TestConfigValues(),
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
		KeyManager:    *signing.TestConfigOverridden(),
		SecretManager: *secrets.TestConfigOverridden(),
		Storage:       *storage.TestConfigOverridden(),

		Port:           "4444",
		CreateTimeout:  25 * time.Minute,
		WorkerTimeout:  25 * time.Minute,
		MinRecords:     3000,
		PaddingRange:   300,
		MaxRecords:     50000,
		TruncateWindow: 3 * time.Hour,
		MinWindowAge:   4 * time.Hour,
		TTL:            558 * time.Hour,
	}
}
