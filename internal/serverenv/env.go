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

// Package serverenv defines common parameters for the sever environment.
package serverenv

import (
	"context"
	"crypto"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// ExporterFunc defines a factory function for creating a context aware metrics exporter.
type ExporterFunc func(context.Context) metrics.Exporter

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	SecretManager    secrets.SecretManager
	KeyManager       signing.KeyManager
	Blobstore        storage.Blobstore
	overrides        map[string]string
	Exporter         metrics.ExporterFromContext
	APIConfigProvier config.Provider
	DB               *database.DB
	Config           interface{}
}

// Option defines function types to modify the ServerEnv on creation.
type Option func(*ServerEnv) *ServerEnv

// New creates a new ServerEnv with the requested options.
func New(ctx context.Context, opts ...Option) *ServerEnv {
	env := &ServerEnv{}
	// A metrics exporter is required, installs the default log based one.
	// Can be overridden by opts.
	env.Exporter = func(ctx context.Context) metrics.Exporter {
		return metrics.NewLogsBasedFromContext(ctx)
	}

	for _, f := range opts {
		env = f(env)
	}

	return env
}

func WithConfig(config interface{}) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.Config = config
		return s
	}
}

func WithPostgresDatabase(db *database.DB) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.DB = db
		return s
	}
}

// WithAPIConfigProvider installs a provider of APIConfig.
func WithAPIConfigProvider(p config.Provider) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.APIConfigProvier = p
		return s
	}
}

// WithMetricsExporter creates an Option to install a different metrics exporter.
func WithMetricsExporter(f metrics.ExporterFromContext) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.Exporter = f
		return s
	}
}

// WithSecretManager creates an Option to install a specific secret manager to use.
func WithSecretManager(sm secrets.SecretManager) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.SecretManager = sm
		return s
	}
}

// WithKeyManager creates an Option to install a specific KeyManager to use for signing requests.
func WithKeyManager(km signing.KeyManager) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.KeyManager = km
		return s
	}
}

// WithBlobStorage creates an Option to install a specific Blob storage system.
func WithBlobStorage(sto storage.Blobstore) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.Blobstore = sto
		return s
	}
}

// GetSignerForKey returns the crypto.Singer implementation to use based on the installed KeyManager.
// If there is no KeyManager installed, this returns an error.
func (s *ServerEnv) GetSignerForKey(ctx context.Context, keyName string) (crypto.Signer, error) {
	if s.KeyManager == nil {
		return nil, fmt.Errorf("no key manager installed, use WithKeyManager when creating the ServerEnv")
	}
	sign, err := s.KeyManager.NewSigner(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("KeyManager.NewSigner: %w", err)
	}
	return sign, nil
}

// MetricsExporter returns a context appropriate metrics exporter.
func (s *ServerEnv) MetricsExporter(ctx context.Context) metrics.Exporter {
	if s.Exporter == nil {
		return nil
	}
	return s.Exporter(ctx)
}
