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
)

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
	case SecretManagerTypeAWSSecretsManager:
		return NewAWSSecretsManager(ctx)
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
