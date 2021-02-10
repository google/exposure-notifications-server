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

package secrets

import (
	"time"
)

// Config represents the config for a secret manager.
type Config struct {
	Type            string        `env:"SECRET_MANAGER, default=IN_MEMORY"`
	SecretsDir      string        `env:"SECRETS_DIR, default=/var/run/secrets"`
	SecretCacheTTL  time.Duration `env:"SECRET_CACHE_TTL, default=5m"`
	SecretExpansion bool          `env:"SECRET_EXPANSION, default=false"`
}
