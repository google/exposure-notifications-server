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

// SecretManagerType represents a type of secret manager.
type SecretManagerType string

const (
	SecretManagerTypeAWSSecretsManager    SecretManagerType = "AWS_SECRETS_MANAGER"
	SecretManagerTypeAzureKeyVault        SecretManagerType = "AZURE_KEY_VAULT"
	SecretManagerTypeGoogleSecretManager  SecretManagerType = "GOOGLE_SECRET_MANAGER"
	SecretManagerTypeGoogleHashiCorpVault SecretManagerType = "HASHICORP_VAULT"
	SecretManagerTypeNoop                 SecretManagerType = "NOOP"
)

// Config represents the config for a secret manager.
type Config struct {
	SecretManagerType SecretManagerType `env:"SECRET_MANAGER, default=GOOGLE_SECRET_MANAGER"`
	SecretsDir        string            `env:"SECRETS_DIR, default=/var/run/secrets"`
	SecretCacheTTL    time.Duration     `env:"SECRET_CACHE_TTL, default=5m"`
	SecretExpansion   bool              `env:"SECRET_EXPANSION, default=false"`
}
