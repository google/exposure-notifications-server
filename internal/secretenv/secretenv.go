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

// Package secretenv wraps envconfig with functionality that can resolve
// secrets from the environment.
//
// This works by transforming the environment variables set by this process and
// then invoking github.com/kelseyhightower/envconfig to resolve environment
// variables to your configuration struct.
//
// If an environment variable starts with 'secret://', for example
//   secret://RESTOFVAR
// Then RESTOFVAR is used as a key to resolve that value in your configured
// secret manager.
// The OS level environment variable is rewritten to be the secret value (which
// only has visability for this running process and any child processes).
//
// If an envirnment variable starts with 'secret://' and ends with '?target=file'
// then, after resolution that value will be written to a file in the SECRETS_DIR
// and the environment variable will be rewritten to be the local path to that file.
//
// This can be used with any secret manager that implements the 'secrets.SecretManager'
// interface.
package secretenv

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/kelseyhightower/envconfig"
)

const (
	// SecretPrefix is the prefix, that if the value of an env var starts with
	// will be resolved through the configured secret store.
	SecretPrefix = "secret://"

	// FileSuffix is the suffix to use, if this secret path should be written to a file.
	// only inteprted on environment variable values that start w/ secret://
	FileSuffix = "?target=file"
)

type envConfig struct {
	SecretDir string `envconfig:"SECRETS_DIR" default:"/var/run/secrets"`
}

// Process uses `envconfig.Process` first, and then crawls the structure again
// looking for values that start with "secret://" and resolving those
// values through the configured `secrets.SecretManager`
func Process(ctx context.Context, prefix string, spec interface{}, sm secrets.SecretManager) error {
	logger := logging.FromContext(ctx)
	// Always load secret config first
	config := &envConfig{}
	err := envconfig.Process("", config)
	if err != nil {
		return fmt.Errorf("error loading secretenv.Config: %w", err)
	}

	err = config.resolveSecrets(ctx, sm)
	if err != nil {
		return err
	}

	err = envconfig.Process(prefix, spec)
	if err != nil {
		return fmt.Errorf("error loading environment variables: %w", err)
	}
	logger.Infof("Loaded environment: %v", spec)

	return nil
}

// filenameForSecret returns the full filepath for the given secret. The
// filename is the sha1 of the secret name, and the path is secretsDir.
func (c *envConfig) filenameForSecret(name string) string {
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(name)))
	return filepath.Join(c.SecretDir, digest)
}

func (c *envConfig) ensureSecretDirExists() error {
	// Create the parent secretsDir with minimal permissions. If the directory
	// does not exist, it is created with 0700 permissions. If the directory
	// exists but has broader permissions that 0700, an error is returned.
	stat, err := os.Stat(c.SecretDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if secretsDir exists: %w", err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(c.SecretDir, 0700); err != nil {
			return fmt.Errorf("failed to create secretsDir: %w", err)
		}
	} else {
		if stat.Mode().Perm() != os.FileMode(0700) {
			return fmt.Errorf("secretsDir exists and is not restricted %v", stat.Mode())
		}
	}
	return nil
}

func checkFileTarget(s string) (string, bool) {
	if strings.HasSuffix(s, FileSuffix) {
		return strings.TrimSuffix(s, FileSuffix), true
	}
	return s, false
}

func secretPath(s string) string {
	if strings.HasPrefix(s, SecretPrefix) {
		return strings.TrimPrefix(s, SecretPrefix)
	}
	return s
}

func (c *envConfig) resolveSecrets(ctx context.Context, sm secrets.SecretManager) error {
	logger := logging.FromContext(ctx)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			logger.Errorf("environment variable unexpected format: %v", e)
			continue
		}
		name := parts[0]
		val := parts[1]
		if strings.HasPrefix(val, SecretPrefix) {
			if sm == nil {
				return fmt.Errorf("environment contains secret values, but there is no secret manager configured")
			}

			val, shouldWriteFile := checkFileTarget(val)
			logger.Infof("resolving secret value for environment variable: %v  toFile: %v", name, shouldWriteFile)

			secretName := secretPath(val)
			secretValue, err := sm.GetSecretValue(ctx, secretName)
			if err != nil {
				return fmt.Errorf("GetSecretValue: %v, error: %w", secretName, err)
			}

			if shouldWriteFile {
				if err := c.ensureSecretDirExists(); err != nil {
					return err
				}
				secretFilePath := c.filenameForSecret(name)
				if err := ioutil.WriteFile(secretFilePath, []byte(secretValue), 0600); err != nil {
					return fmt.Errorf("failed to write secret %q to file: %w", name, err)
				}
				logger.Infof("wrote secret file for %v", name)
				secretValue = secretFilePath
			}
			// Replace the value of the envrionment variable with the either the resolved secret value
			// or the file path to the secret value saved on the filesystem.
			// This takes effect for this process and any child processes only.
			// When envconfig is used to load a variable if
			//  DB_PASS was "secret://pathto/databasepassword"
			//  DB_PASS will not be the actual database password for the application to consume.
			os.Setenv(name, secretValue)
		}
	}
	return nil
}
