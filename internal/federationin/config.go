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

package federationin

import (
	"regexp"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
)

const (
	// DefaultAudience is the default OIDC audience.
	DefaultAudience = "https://exposure-notifications-server/federation"
)

var (
	// ValidAudienceStr is the regexp string of a valid audience string.
	ValidAudienceStr = `\Ahttps://.*\z`
	// ValidAudienceRegexp is the compiled regexp of a valid audience string.
	ValidAudienceRegexp = regexp.MustCompile(ValidAudienceStr)
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

// Config is the configuration for federation-pull components (data pulled from other servers).
type Config struct {
	Database      database.Config
	SecretManager secrets.Config

	Port           string        `env:"PORT, default=8080"`
	Timeout        time.Duration `env:"RPC_TIMEOUT, default=10m"`
	TruncateWindow time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// TLSSkipVerify, if set to true, causes the server certificate to not be
	// verified. This is typically used when testing locally with self-signed
	// certificates.
	TLSSkipVerify bool `env:"TLS_SKIP_VERIFY"`

	// TLSCertFile points to an optional cert file that will be appended to the
	// system certificates.
	TLSCertFile string `env:"TLS_CERT_FILE"`

	// CredentialsFile points to a JSON credentials file. If running on Managed
	// Cloud Run, or if using $GOOGLE_APPLICATION_CREDENTIALS, leave this value
	// empty.
	CredentialsFile string `env:"CREDENTIALS_FILE"`
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

		Port:            "8080",
		Timeout:         10 * time.Minute,
		TruncateWindow:  1 * time.Hour,
		TLSSkipVerify:   false,
		TLSCertFile:     "",
		CredentialsFile: "",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		Database:      *database.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),

		Port:            "5555",
		Timeout:         15 * time.Minute,
		TruncateWindow:  11 * time.Hour,
		TLSSkipVerify:   true,
		TLSCertFile:     "/var/foo",
		CredentialsFile: "/var/bar",
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT":             "5555",
		"RPC_TIMEOUT":      "15m",
		"TRUNCATE_WINDOW":  "11h",
		"TLS_SKIP_VERIFY":  "true",
		"TLS_CERT_FILE":    "/var/foo",
		"CREDENTIALS_FILE": "/var/bar",
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

		Port:            "4444",
		Timeout:         25 * time.Minute,
		TruncateWindow:  21 * time.Hour,
		TLSSkipVerify:   true,
		TLSCertFile:     "/etc/foo",
		CredentialsFile: "/etc/bar",
	}
}
