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

// Package authorizedapp handles allowed applications.
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
