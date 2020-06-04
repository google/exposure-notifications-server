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

package authorizedapp

import (
	"time"
)

type Config struct {
	// CacheDuration is the amount of time AuthorizedApp should be cached before
	// being re-read from their provider.
	CacheDuration time.Duration `env:"AUTHORIZED_APP_CACHE_DURATION,default=5m"`
}

// AuthorizedApp implements an interface for setup.
func (c *Config) AuthorizedApp() *Config {
	return c
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		CacheDuration: 5 * time.Minute,
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		CacheDuration: 10 * time.Minute,
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	return map[string]string{
		"AUTHORIZED_APP_CACHE_DURATION": "10m",
	}
}

// TestConfigOverridden returns a configuration with non-default values set. It
// should only be used for testing.
func TestConfigOverridden() *Config {
	return &Config{
		CacheDuration: 30 * time.Minute,
	}
}
