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
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/google/exposure-notifications-server/internal/signing"
	"github.com/google/exposure-notifications-server/internal/storage"
)

const (
	portEnvVar  = "PORT"
	defaultPort = "8080"
	// SecretPostfix designates that environment variable ending with this value
	SecretPostfix = "_SECRET"

	// defaultSecretsDir is the default directory where secrets are stored.
	defaultSecretsDir = "/var/run/secrets"
)

// ExporterFunc defines a factory function for creating a context aware metrics exporter.
type ExporterFunc func(context.Context) metrics.Exporter

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	Port          string
	SecretManager secrets.SecretManager
	KeyManager    signing.KeyManager
	Blobstore     storage.Blobstore
	overrides     map[string]string
	Exporter      metrics.ExporterFromContext

	// secretsDir is the path to the directory where secrets are saved.
	secretsDir string
}

// Option defines function types to modify the ServerEnv on creation.
type Option func(*ServerEnv) *ServerEnv

// New creates a new ServerEnv with the requested options.
func New(ctx context.Context, opts ...Option) *ServerEnv {
	env := &ServerEnv{Port: defaultPort}
	// A metrics exporter is required, installs the default log based one.
	// Can be overridden by opts.
	env.Exporter = func(ctx context.Context) metrics.Exporter {
		return metrics.NewLogsBasedFromContext(ctx)
	}

	logger := logging.FromContext(ctx)

	env.secretsDir = defaultSecretsDir

	if override := os.Getenv(portEnvVar); override != "" {
		env.Port = override
	}
	logger.Infof("using port %v (override with $%v)", env.Port, portEnvVar)

	for _, f := range opts {
		env = f(env)
	}

	return env
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

func (s *ServerEnv) getSecretValue(ctx context.Context, envVar string) (string, error) {
	logger := logging.FromContext(ctx)

	eVal := os.Getenv(envVar)
	if s.SecretManager == nil {
		logger.Warnf("resolve %v with local environment variable, no secret manager not configured", envVar)
		return eVal, nil
	}

	secretVar := envVar + SecretPostfix
	secretLocation := os.Getenv(secretVar)
	if secretLocation == "" {
		logger.Warnf("resolving %v with local environment value %v is unset.", envVar, secretVar)
		return eVal, nil
	}

	// Resolve through the installed secret manager.
	plaintext, err := s.SecretManager.GetSecretValue(ctx, secretLocation)
	if err != nil {
		return "", fmt.Errorf("failed to access secret value for %v: %w", secretLocation, err)
	}
	logger.Infof("loaded %v from secret %v", envVar, secretLocation)
	return plaintext, nil
}

// filenameForSecret returns the full filepath for the given secret. The
// filename is the sha1 of the secret name, and the path is secretsDir.
func (s *ServerEnv) filenameForSecret(name string) string {
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(name)))
	return filepath.Join(s.secretsDir, digest)
}

// ResolveSecretEnv will either resolve a local environment variable
// by name, or if the same env var with a postfix of "_SECRET" is set
// then the value will be resolved as a key into a secret store.
func (s *ServerEnv) ResolveSecretEnv(ctx context.Context, envVar string) (string, error) {
	if val, ok := s.overrides[envVar]; ok {
		return val, nil
	}
	return s.getSecretValue(ctx, envVar)
}

// WriteSecretToFile attempt to resolve an environment variable with a name of
// "envVar_SECRET" by calling the secret manager and writing the returned value
// to a file in a temporary directory. The full filepath is returned.
func (s *ServerEnv) WriteSecretToFile(ctx context.Context, envVar string) (string, error) {
	logger := logging.FromContext(ctx)

	// Create the parent secretsDir with minimal permissions. If the directory
	// does not exist, it is created with 0700 permissions. If the directory
	// exists but has broader permissions that 0700, an error is returned.
	stat, err := os.Stat(s.secretsDir)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check if secretsDir exists: %w", err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(s.secretsDir, 0700); err != nil {
			return "", fmt.Errorf("failed to create secretsDir: %w", err)
		}
	} else {
		if stat.Mode().Perm() != os.FileMode(0700) {
			return "", fmt.Errorf("secretsDir exists and is not restricted %v", stat.Mode())
		}
	}

	secretVal, err := s.ResolveSecretEnv(ctx, envVar)
	if err != nil {
		return "", err
	}

	secretPath := s.filenameForSecret(envVar)
	if err := ioutil.WriteFile(secretPath, []byte(secretVal), 0600); err != nil {
		return "", fmt.Errorf("failed to write secret %q to file: %w", envVar, err)
	}

	logger.Infof("Wrote secret value for %v to file", envVar)
	return secretPath, nil
}

// Set overrides the usual lookup for name so that value is always returned.
func (s *ServerEnv) Set(name, value string) {
	if s.overrides == nil {
		s.overrides = map[string]string{}
	}
	s.overrides[name] = value
}

// MetricsExporter returns a context appropriate metrics exporter.
func (s *ServerEnv) MetricsExporter(ctx context.Context) metrics.Exporter {
	if s.Exporter == nil {
		return nil
	}
	return s.Exporter(ctx)
}

// ParseDuration parses a duration string stored in the named environment
// variable. If the variable's values is empty or cannot be parsed as a
// duration, the default value is returned instead.
func ParseDuration(ctx context.Context, name string, defaultValue time.Duration) time.Duration {
	val := os.Getenv(name)
	if val == "" {
		return defaultValue
	}
	dur, err := time.ParseDuration(val)
	if err != nil {
		logging.FromContext(ctx).Warnf("Failed to parse $%s value %q, using default %s", name, val, defaultValue)
		return defaultValue
	}
	return dur
}

// ParseBool parses a bool string stored in the named environment variable. If
// the variable's values is empty or cannot be parsed as a bool, the default
// value is returned instead.
func ParseBool(ctx context.Context, name string, defaultValue bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return defaultValue
	}

	b, err := strconv.ParseBool(val)
	if err != nil {
		logging.FromContext(ctx).Warnf("Failed to parse $%s value %q, using default %t", name, val, defaultValue)
		return defaultValue
	}
	return b
}
