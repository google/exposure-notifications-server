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

// ServerEnv represents latent environment configuration for servers in this application.
type ServerEnv struct {
	port     string
	smClient *secretmanager.Client
}

type ServerEnvOption func(context.Context, *ServerEnv) (*ServerEnv, error)

func New(ctx context.Context, opts ...ServerEnvOption) (*ServerEnv, error) {
	env := &ServerEnv{
		port: defaultPort,
	}

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

func WithSecretManager(ctx context.Context, s *ServerEnv) (*ServerEnv, error) {
	use := os.Getenv(useSecretManagerEnvVar)
	if use == "" {
		logging.FromContext(ctx).Warnf("secret manager for environment variable resolving not enabled, set %v to enable", useSecretManagerEnvVar)
	}

	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %v", err)
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

	secretVar := fmt.Sprintf("%v%v", envVar, SecretPostfix)
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
		return "", fmt.Errorf("failed to access secret version for %v: %v", secretLocation, err)
	}

	plaintext := string(result.Payload.Data)
	return plaintext, nil
}

// ResolveSecretEnv will either resolve a local environment variable
// by name, or if the same env var with a postfix of "_SECRET" is set
// then the value will be resolved as a key into a secret store.
func (s *ServerEnv) ResolveSecretEnv(ctx context.Context, envVar string) (string, error) {
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

	fName := fmt.Sprintf("/tmp/secret-%v", envVar)
	data := []byte(secretVal)
	err = ioutil.WriteFile(fName, data, 0640)
	if err != nil {
		return "", fmt.Errorf("unable to write secret for %v to file: %v", envVar, err)
	}
	logger.Infof("Wrote secret value for %v to file", envVar)
	return fName, nil
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
