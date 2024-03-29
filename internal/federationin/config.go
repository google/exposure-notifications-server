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

package federationin

import (
	"regexp"
	"time"

	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
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
var (
	_ setup.DatabaseConfigProvider              = (*Config)(nil)
	_ setup.SecretManagerConfigProvider         = (*Config)(nil)
	_ setup.ObservabilityExporterConfigProvider = (*Config)(nil)
)

// Config is the configuration for federation-pull components (data pulled from other servers).
type Config struct {
	Database              database.Config
	SecretManager         secrets.Config
	ObservabilityExporter observability.Config

	Port                         string        `env:"PORT, default=8080"`
	Timeout                      time.Duration `env:"RPC_TIMEOUT, default=10m"`
	TruncateWindow               time.Duration `env:"TRUNCATE_WINDOW, default=1h"`
	MaxIntervalAge               time.Duration `env:"MAX_INTERVAL_AGE_ON_PUBLISH, default=360h"`
	MaxMagnitudeSymptomOnsetDays uint          `env:"MAX_SYMPTOM_ONSET_DAYS, default=14"`

	// Flags for local development and testing. This will cause still valid keys
	// to not be embargoed.
	// Normally "still valid" keys can be accepted, but are embargoed.
	ReleaseSameDayKeys bool `env:"DEBUG_RELEASE_SAME_DAY_KEYS"`

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

	// Sync configuration.
	// If accepted, both self report and recursive will be sent as clinical,
	// otherwise they will be dropped.
	AcceptSelfReport bool `env:"ACCEPT_SELF_REPORT, default=false"`
	AcceptRecursive  bool `env:"ACCEPT_RECURSIVE, default=false"`
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
