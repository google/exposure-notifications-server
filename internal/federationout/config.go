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
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// Compile-time check to assert this config matches requirements.
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)
var _ setup.ObservabilityExporterConfigProvider = (*Config)(nil)

// Config is the configuration for the federation components (data sent to other servers).
type Config struct {
	Database              database.Config
	SecretManager         secrets.Config
	ObservabilityExporter observability.Config

	Port           string        `env:"PORT, default=8080"`
	MaxRecords     uint32        `env:"MAX_RECORDS, default=500"`
	Timeout        time.Duration `env:"RPC_TIMEOUT, default=5m"`
	TruncateWindow time.Duration `env:"TRUNCATE_WINDOW, default=1h"`

	// AllowAnyClient, if true, removes authentication requirements on the
	// federation endpoint. In practice, this is only useful in local testing.
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

func (c *Config) ObservabilityExporterConfig() *observability.Config {
	return &c.ObservabilityExporter
}
