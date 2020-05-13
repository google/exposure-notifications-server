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

package federation

import (
	"regexp"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
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

// Config is the configuration for the federation components (data sent to other servers).
type Config struct {
	Port     string        `envconfig:"PORT" default:"8080"`
	Timeout  time.Duration `envconfig:"RPC_TIMEOUT" default:"5m"`
	Database *database.Config

	// AllowAnyClient, if true, removes authentication requirements on the federation endpoint.
	// In practise, this is only useful in local testing.
	AllowAnyClient bool `envconfig:"ALLOW_ANY_CLIENT" default:"false"`

	// TLSCertFile is the certificate file to use if TLS encryption is enabled on the server.
	// If present, TLSKeyFile must also be present. These settings should be left blank on
	// Managed Cloud Run where the TLS termination is handled by the environment.
	TLSCertFile string `envconfig:"TLS_CERT_FILE"`
	TLSKeyFile  string `envconfig:"TLS_KEY_FILE"`
}

func (c *Config) DB() *database.Config {
	return c.Database
}

// PullConfig is the configuration for federation-pull components (data pulled from other servers).
type PullConfig struct {
	Database *database.Config
	Port     string        `envconfig:"PORT" default:"8080"`
	Timeout  time.Duration `envconfig:"RPC_TIMEOUT" default:"10m"`

	// TLSSkipVerify, if set to true, causes the server certificate to not be verified.
	// This is typically used when testing locally with self-signed certificates.
	TLSSkipVerify bool `envconfig:"TLS_SKIP_VERIFY" default:"false"`

	// TLSCertFile points to an optional cert file that will be appended to the system certificates.
	TLSCertFile string `envconfig:"TLS_CERT_FILE"`

	// CredentialsFile points to a JSON credentials file. If running on Managed Cloud Run,
	// or if using $GOOGLE_APPLICATION_CREDENTIALS, leave this value empty.
	CredentialsFile string `envconfig:"CREDENTIALS_FILE"`
}

func (c *PullConfig) DB() *database.Config {
	return c.Database
}
