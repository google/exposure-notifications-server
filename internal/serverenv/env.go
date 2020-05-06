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

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

const (
	portEnvVar             = "PORT"
	defaultPort            = "8080"
	useSecretManagerEnvVar = "USE_SECRET_MANAGER"
	// SecretPostfix designates that environment variable ending with this value
	SecretPostfix = "_SECRET"
)

// ExporterFunc defines a factory function for creating a context aware metrics exporter.
type ExporterFunc func(context.Context) metrics.Exporter

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	port      string
	smClient  *secretmanager.Client
	overrides map[string]string
	exporter  ExporterFunc
}

// Option defines function types to modify the ServerEnv on creation.
type Option func(context.Context, *ServerEnv) (*ServerEnv, error)

// New creates a new ServerEnv with the requested options.
func New(ctx context.Context, opts ...Option) (*ServerEnv, error) {
	env := &ServerEnv{
		port: defaultPort,
	}
	// The default is logs based metrics. This is applied before the opts, which
	// may override the exporter implementation.
	env, _ = WithLogsBasedMetrics(ctx, env)

	logger := logging.FromContext(ctx)

	if override := os.Getenv(portEnvVar); override != "" {
		env.port = override
	}
	logger.Infof("using port %v (override with $%v)", env.port, portEnvVar)

	for _, f := range opts {
		var err error
		env, err = f(ctx, env)
		if err != nil {
			return nil, err
		}
	}

	return env, nil
}

// WithLogsBasedMetrics installs a factory function that uses the metrics.NewLogsBasedFromContext
// to return a metrics exporter.
// The context passed in here isn't used.
func WithLogsBasedMetrics(unused context.Context, s *ServerEnv) (*ServerEnv, error) {
	s.exporter = func(ctx context.Context) metrics.Exporter {
		return metrics.NewLogsBasedFromContext(ctx)
	}
	return s, nil
}

func WithSecretManager(ctx context.Context, s *ServerEnv) (*ServerEnv, error) {
	use := os.Getenv(useSecretManagerEnvVar)
	if use == "" {
		logging.FromContext(ctx).Warnf("secret manager for environment variable resolving not enabled, set %v to enable", useSecretManagerEnvVar)
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}
	s.smClient = client
	return s, nil
}

func (s *ServerEnv) Port() string {
	return s.port
}

func (s *ServerEnv) getSecretValue(ctx context.Context, envVar string) (string, error) {
	logger := logging.FromContext(ctx)

	eVal := os.Getenv(envVar)
	if s.smClient == nil {
		logger.Warnf("resolve %v with local environment variable, secret manager not configured", envVar)
		return eVal, nil
	}

	secretVar := envVar + SecretPostfix
	secretLocation := os.Getenv(secretVar)
	if secretLocation == "" {
		logger.Warnf("resolving %v with local environment value %v is unset.", envVar, secretVar)
		return eVal, nil
	}

	// Build the request.
	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretLocation,
	}

	// Call the API.
	result, err := s.smClient.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return "", fmt.Errorf("failed to access secret version for %v: %w", secretLocation, err)
	}
	logger.Infof("loaded %v from secret %v", envVar, secretLocation)

	plaintext := string(result.Payload.Data)
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
