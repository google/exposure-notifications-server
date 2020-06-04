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

package monolith

import (
	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/cleanup"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/export"
	"github.com/google/exposure-notifications-server/internal/federationin"
	"github.com/google/exposure-notifications-server/internal/publish"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

var _ setup.AuthorizedAppConfigProvider = (*Config)(nil)
var _ setup.BlobstoreConfigProvider = (*Config)(nil)
var _ setup.DatabaseConfigProvider = (*Config)(nil)
var _ setup.KeyManagerConfigProvider = (*Config)(nil)
var _ setup.SecretManagerConfigProvider = (*Config)(nil)

type Config struct {
	AuthorizedApp authorizedapp.Config
	Storage       storage.Config
	Cleanup       cleanup.Config
	Database      database.Config
	Export        export.Config
	FederationIn  federationin.Config
	KeyManager    signing.Config
	Publish       publish.Config
	SecretManager secrets.Config

	Port string `env:"PORT, default=8080"`
}

func (c *Config) AuthorizedAppConfig() *authorizedapp.Config {
	return &c.AuthorizedApp
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
		AuthorizedApp: *authorizedapp.TestConfigDefaults(),
		Storage:       *storage.TestConfigDefaults(),
		Cleanup:       *cleanup.TestConfigDefaults(),
		Database:      *database.TestConfigDefaults(),
		Export:        *export.TestConfigDefaults(),
		FederationIn:  *federationin.TestConfigDefaults(),
		KeyManager:    *signing.TestConfigDefaults(),
		Publish:       *publish.TestConfigDefaults(),
		SecretManager: *secrets.TestConfigDefaults(),

		Port: "8080",
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	c := &Config{
		AuthorizedApp: *authorizedapp.TestConfigValued(),
		Storage:       *storage.TestConfigValued(),
		Cleanup:       *cleanup.TestConfigValued(),
		Database:      *database.TestConfigValued(),
		Export:        *export.TestConfigValued(),
		FederationIn:  *federationin.TestConfigValued(),
		KeyManager:    *signing.TestConfigValued(),
		Publish:       *publish.TestConfigValued(),
		SecretManager: *secrets.TestConfigValued(),

		Port: "5555",
	}

	// These fields share an environment variable name, so they get overridden in
	// the map below. Last one wins, which is publish.
	c.Export.TruncateWindow = c.Publish.TruncateWindow
	c.FederationIn.TruncateWindow = c.Publish.TruncateWindow
	return c
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	m := map[string]string{
		"PORT": "5555",
	}

	embedded := []map[string]string{
		authorizedapp.TestConfigValues(),
		storage.TestConfigValues(),
		cleanup.TestConfigValues(),
		database.TestConfigValues(),
		export.TestConfigValues(),
		federationin.TestConfigValues(),
		signing.TestConfigValues(),
		publish.TestConfigValues(),
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
		Storage:       *storage.TestConfigOverridden(),
		Cleanup:       *cleanup.TestConfigOverridden(),
		Database:      *database.TestConfigOverridden(),
		Export:        *export.TestConfigOverridden(),
		FederationIn:  *federationin.TestConfigOverridden(),
		KeyManager:    *signing.TestConfigOverridden(),
		Publish:       *publish.TestConfigOverridden(),
		SecretManager: *secrets.TestConfigOverridden(),

		Port: "4444",
	}
}
