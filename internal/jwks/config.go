// Copyright 2021 Google LLC
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

package jwks

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/setup"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

var (
	_ setup.DatabaseConfigProvider      = (*Config)(nil)
	_ setup.SecretManagerConfigProvider = (*Config)(nil)
)

type Config struct {
	Database      database.Config
	SecretManager secrets.Config

	Port string `env:"PORT, default=8080"`

	// MaxRuntime is how long an individual handler should run.
	MaxRuntime time.Duration `env:"MAX_RUNTIME, default=10m"`

	// RequestTimeout is the client per-request timeout when accessing
	// remote JWKS documents.
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT, default=30s"`

	KeyCleanupTTL time.Duration `env:"HEALTH_AUTHORITY_KEY_CLEANUP_TTL, default=720h"` // 30 days

	// MaxWorkers is the number of parallel JWKS updates that can occur.
	MaxWorkers uint `env:"MAX_WORKERS, default=5"`
}

func (c *Config) DatabaseConfig() *database.Config {
	return &c.Database
}

func (c *Config) SecretManagerConfig() *secrets.Config {
	return &c.SecretManager
}
