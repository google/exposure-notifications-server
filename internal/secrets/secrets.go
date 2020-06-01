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

// Package secrets defines a minimum abstract interface for a secret manager.
// Allows for a different implementation to be bound within the servernv.ServeEnv
package secrets

import (
	"context"
	"fmt"
	"time"
)

// SecretManagerType represents a type of secret manager.
type SecretManagerType string

const (
	SecretManagerTypeAzureKeyVault        SecretManagerType = "AZURE_KEY_VAULT"
	SecretManagerTypeGoogleSecretManager  SecretManagerType = "GOOGLE_SECRET_MANAGER"
	SecretManagerTypeGoogleHashiCorpVault SecretManagerType = "HASHICORP_VAULT"
	SecretManagerTypeNoop                 SecretManagerType = "NOOP"
)

// Config represents the config for a secret manager.
type Config struct {
	SecretManagerType SecretManagerType `envconfig:"SECRET_MANAGER" default:"GOOGLE_SECRET_MANAGER"`
	SecretCacheTTL    time.Duration     `envconfig:"SECRET_CACHE_TTL" default:"5m"`
}

// SecretManager defines the minimum shared functionality for a secret manager
// used by this application.
type SecretManager interface {
	GetSecretValue(ctx context.Context, name string) (string, error)
}

// SecretManagerFunc is a func that returns a secret manager or error.
type SecretManagerFunc func(ctx context.Context) (SecretManager, error)

// SecretManagerFor returns the secret manager for the given type, or an error
// if one does not exist.
func SecretManagerFor(ctx context.Context, typ SecretManagerType) (SecretManager, error) {
	switch typ {
	case SecretManagerTypeAzureKeyVault:
		return NewAzureKeyVault(ctx)
	case SecretManagerTypeGoogleSecretManager:
		return NewGoogleSecretManager(ctx)
	case SecretManagerTypeGoogleHashiCorpVault:
		return NewHashiCorpVault(ctx)
	case SecretManagerTypeNoop:
		return NewNoop(ctx)
	}

	return nil, fmt.Errorf("unknown secret manager type: %v", typ)
}
