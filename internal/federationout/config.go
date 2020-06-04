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

package federationout

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config is the configuration for the federation components (data sent to other servers).
type Config struct {
	Database      database.Config
	SecretManager secrets.Config

	Port           string        `env:"PORT, default=8080"`
	Timeout        time.Duration `env:"RPC_TIMEOUT, default=5m"`
	TruncateWindow time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// AllowAnyClient, if true, removes authentication requirements on the
	// federation endpoint. In practise, this is only useful in local testing.
	AllowAnyClient bool `env:"ALLOW_ANY_CLIENT"`

	// TLSCertFile is the certificate file to use if TLS encryption is enabled on
	// the server. If present, TLSKeyFile must also be present. These settings
	// should be left blank on Managed Cloud Run where the TLS termination is
	// handled by the environment.
	TLSCertFile string `env:"TLS_CERT_FILE"`
	TLSKeyFile  string `env:"TLS_KEY_FILE"`
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

		Port:           "8080",
		Timeout:        5 * time.Minute,
		TruncateWindow: 1 * time.Hour,
		AllowAnyClient: false,
		TLSCertFile:    "",
		TLSKeyFile:     "",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Database:      *database.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),

		Port:           "5555",
		Timeout:        15 * time.Minute,
		TruncateWindow: 11 * time.Hour,
		AllowAnyClient: true,
		TLSCertFile:    "/var/foo",
		TLSKeyFile:     "/var/bar",
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":             "5555",
		"RPC_TIMEOUT":      "15m",
		"TRUNCATE_WINDOW":  "11h",
		"ALLOW_ANY_CLIENT": "true",
		"TLS_CERT_FILE":    "/var/foo",
		"TLS_KEY_FILE":     "/var/bar",
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

		Port:           "4444",
		Timeout:        25 * time.Minute,
		TruncateWindow: 21 * time.Hour,
		AllowAnyClient: true,
		TLSCertFile:    "/etc/foo",
		TLSKeyFile:     "/etc/bar",
	}
}
