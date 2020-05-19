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
	"github.com/google/exposure-notifications-server/internal/setup"
)

// Compile-time check to assert this config matches requirements.
var _ setup.AuthorizedAppConfigProvider = (*Config)(nil)
var _ setup.DBConfigProvider = (*Config)(nil)

// Config represents the configuration and associated environment variables for
// the publish components.
type Config struct {
	Port               string        `envconfig:"PORT" default:"8080"`
	MinRequestDuration time.Duration `envconfig:"TARGET_REQUEST_DURATION" default:"5s"`
	MaxKeysOnPublish   int           `envconfig:"MAX_KEYS_ON_PUBLISH" default:"15"`
	MaxIntervalAge     time.Duration `envconfig:"MAX_INTERVAL_AGE_ON_PUBLISH" default:"360h"`
	TruncateWindow     time.Duration `envconfig:"TRUNCATE_WINDOW" default:"1h"`

	// Bypass flags for local development and testing.
	BypassSafetyNet   bool `envconfig:"BYPASS_SAFETYNET"`
	BypassDeviceCheck bool `envconfig:"BYPASS_DEVICECHECK"`
	DebugAPIResponses bool `envconfig:"DEBUG_API_RESPONSES"`

	AuthorizedApp *authorizedapp.Config
	Database      *database.Config
}

// AuthorizedApp returns the configuration for authorizedapp.
func (c *Config) AuthorizedAppConfig() *authorizedapp.Config {
	return c.AuthorizedApp
}

// DB returns the configuration for the databse.
func (c *Config) DB() *database.Config {
	return c.Database
}
