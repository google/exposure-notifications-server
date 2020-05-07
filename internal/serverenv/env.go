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
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/internal/secrets"
)

const (
	portEnvVar  = "PORT"
	defaultPort = "8080"
	// SecretPostfix designates that environment variable ending with this value
	SecretPostfix = "_SECRET"
)

// ExporterFunc defines a factory function for creating a context aware metrics exporter.
type ExporterFunc func(context.Context) metrics.Exporter

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	port          string
	secretManager secrets.SecretManager // Optional
	overrides     map[string]string
	exporter      metrics.ExporterFromContext
}

// Option defines function types to modify the ServerEnv on creation.
type Option func(*ServerEnv) *ServerEnv

// New creates a new ServerEnv with the requested options.
func New(ctx context.Context, opts ...Option) *ServerEnv {
	env := &ServerEnv{port: defaultPort}
	// A metrics exporter is required, installs the default log based one.
	// Can be overridden by opts.
	env.exporter = func(ctx context.Context) metrics.Exporter {
		return metrics.NewLogsBasedFromContext(ctx)
	}

	logger := logging.FromContext(ctx)

	if override := os.Getenv(portEnvVar); override != "" {
		env.port = override
	}
	logger.Infof("using port %v (override with $%v)", env.port, portEnvVar)

	for _, f := range opts {
		env = f(env)
	}

	return env
}

// WithMetricsExporter creates an Option to install a different metrics exporter.
func WithMetricsExporter(f metrics.ExporterFromContext) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.exporter = f
		return s
	}
}

// WithSecretManager creates an Option to in stall a specific secret manager to use.
func WithSecretManager(sm secrets.SecretManager) Option {
	return func(s *ServerEnv) *ServerEnv {
		s.secretManager = sm
		return s
	}
}

// Port returns the port that a server should listen on.
func (s *ServerEnv) Port() string {
	return s.port
}

func (s *ServerEnv) getSecretValue(ctx context.Context, envVar string) (string, error) {
	logger := logging.FromContext(ctx)

	eVal := os.Getenv(envVar)
	if s.secretManager == nil {
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
	plaintext, err := s.secretManager.GetSecretValue(ctx, secretLocation)
	if err != nil {
		return "", fmt.Errorf("failed to access secret value for %v: %w", secretLocation, err)
	}
	logger.Infof("loaded %v from secret %v", envVar, secretLocation)
	return plaintext, nil
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
// to a file in /tmp. The filename is returned.
func (s *ServerEnv) WriteSecretToFile(ctx context.Context, envVar string) (string, error) {
	logger := logging.FromContext(ctx)

	secretVal, err := s.getSecretValue(ctx, envVar)
	if err != nil {
		return "", err
	}

	fName := "/tmp/secret-%v" + envVar
	data := []byte(secretVal)
	err = ioutil.WriteFile(fName, data, 0600)
	if err != nil {
		return "", fmt.Errorf("unable to write secret for %v to file: %w", envVar, err)
	}
	logger.Infof("Wrote secret value for %v to file", envVar)
	return fName, nil
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
	if s.exporter == nil {
		return nil
	}
	return s.exporter(ctx)
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
