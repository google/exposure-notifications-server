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

// Package serverenv defines common parameters for the sever environment.
package serverenv

import (
	"context"
	"crypto"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/keys"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/google/exposure-notifications-server/pkg/secrets"
)

// ExporterFunc defines a factory function for creating a context aware metrics exporter.
type ExporterFunc func(context.Context) metrics.Exporter

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	authorizedAppProvider authorizedapp.Provider
	blobstore             storage.Blobstore
	database              *database.DB
	exporter              metrics.ExporterFromContext
	keyManager            keys.KeyManager
	secretManager         secrets.SecretManager
	observabilityExporter observability.Exporter
}

// Option defines function types to modify the ServerEnv on creation.
type Option func(*ServerEnv) *ServerEnv

// New creates a new ServerEnv with the requested options.
func New(ctx context.Context, opts ...Option) *ServerEnv {
	env := &ServerEnv{}
	// A metrics exporter is required, installs the default log based one.
	// Can be overridden by opts.
	env.exporter = func(ctx context.Context) metrics.Exporter {
		return metrics.NewLogsBasedFromContext(ctx)
	}

	for _, f := range opts {
		env = f(env)
	}

	return env
}

// WithDatabase attached a database to the environment.
func WithDatabase(db *database.DB) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.database = db
		return s
	}
}

// WithAuthorizedAppProvider installs a provider for an authorized app.
func WithAuthorizedAppProvider(p authorizedapp.Provider) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.authorizedAppProvider = p
		return s
	}
}

// WithMetricsExporter creates an Option to install a different metrics exporter.
func WithMetricsExporter(f metrics.ExporterFromContext) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.exporter = f
		return s
	}
}

// WithSecretManager creates an Option to install a specific secret manager to use.
func WithSecretManager(sm secrets.SecretManager) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.secretManager = sm
		return s
	}
}

// WithKeyManager creates an Option to install a specific KeyManager to use for signing requests.
func WithKeyManager(km keys.KeyManager) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.keyManager = km
		return s
	}
}

// WithBlobStorage creates an Option to install a specific Blob storage system.
func WithBlobStorage(sto storage.Blobstore) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.blobstore = sto
		return s
	}
}

// WithObservabilityExporter creates an Option to install a specific observability exporter system.
func WithObservabilityExporter(oe observability.Exporter) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.observabilityExporter = oe
		return s
	}
}

func (s *ServerEnv) SecretManager() secrets.SecretManager {
	return s.secretManager
}

func (s *ServerEnv) KeyManager() keys.KeyManager {
	return s.keyManager
}

func (s *ServerEnv) Blobstore() storage.Blobstore {
	return s.blobstore
}

func (s *ServerEnv) AuthorizedAppProvider() authorizedapp.Provider {
	return s.authorizedAppProvider
}

func (s *ServerEnv) Database() *database.DB {
	return s.database
}

func (s *ServerEnv) ObservabilityExporter() observability.Exporter {
	return s.observabilityExporter
}

func (s *ServerEnv) GetKeyManager() keys.KeyManager {
	return s.keyManager
}

// GetSignerForKey returns the crypto.Singer implementation to use based on the installed KeyManager.
// If there is no KeyManager installed, this returns an error.
func (s *ServerEnv) GetSignerForKey(ctx context.Context, keyName string) (crypto.Signer, error) {
	if s.keyManager == nil {
		return nil, fmt.Errorf("no key manager installed, use WithKeyManager when creating the ServerEnv")
	}
	sign, err := s.keyManager.NewSigner(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("KeyManager.NewSigner: %w", err)
	}
	return sign, nil
}

// MetricsExporter returns a context appropriate metrics exporter.
func (s *ServerEnv) MetricsExporter(ctx context.Context) metrics.Exporter {
	if s.exporter == nil {
		return nil
	}
	return s.exporter(ctx)
}

// Close shuts down the server env, closing database connections, etc.
func (s *ServerEnv) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}

	if s.database != nil {
		s.database.Close(ctx)
	}

	if s.observabilityExporter != nil {
		if err := s.observabilityExporter.Close(); err != nil {
			return nil
		}
	}

	return nil
}
