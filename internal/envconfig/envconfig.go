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

// Package envconfig resolves configuration and secret values from environment
// variables. This works by transforming the environment variables and then
// invoking github.com/kelseyhightower/envconfig to map the environment
// variables into a provided struct.
//
// If an environment variable begins with "secret://", the remaining string bits
// are used to resolve the value in the provided SecretManager. For example:
//
//     FOO=secret://foo/bar/baz => secretmanager.GetSecretValue("foo/bar/baz")
//
// The environment variables are rewritten to be the secret value, but this is
// only visible within the running process and any child processes.
//
// If an environment variable secret ends with "?target=file" then the resulting
// secret value is written to SECRETS_DIR and the environment variable is
// updated to be the local path to that file.
//
// This can be used with any secret manager that implements the
// 'secrets.SecretManager' interface.
package envconfig

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
	kenvconfig "github.com/kelseyhightower/envconfig"
)

const (
	// SecretPrefix is the prefix, that if the value of an env var starts with
	// will be resolved through the configured secret store.
	SecretPrefix = "secret://"

	// FileSuffix is the suffix to use, if this secret path should be written to a file.
	// only interpreted on environment variable values that start w/ secret://
	FileSuffix = "?target=file"
)

// BaseConfig is the default base configuration.
type BaseConfig struct {
	// SecretsDir is the base directory where secrets are stored.
	SecretsDir string `envconfig:"SECRETS_DIR" default:"/var/run/secrets"`
}

// Process processes the provided spec, resolving any values that match the
// given struct tags for `envconfig`. If any values start with "secret://",
// those values are resolved using the provided SecretManager. SecretManager can
// be nil if no secrets are being resolved.
func Process(ctx context.Context, spec interface{}, sm secrets.SecretManager) error {
	logger := logging.FromContext(ctx)

	// First resolve the base configuration to get the configured secrets
	// directory and any other base values.
	var config BaseConfig
	if err := kenvconfig.Process("", &config); err != nil {
		return fmt.Errorf("failed to process base config: %w", err)
	}

	// Now resolve any secrets in the environment.
	if err := resolveSecrets(ctx, sm, config.SecretsDir); err != nil {
		return err
	}

	// Now process the updated environment into the spec interface.
	if err := kenvconfig.Process("", spec); err != nil {
		return fmt.Errorf("failed to process given config: %w", err)
	}
	logger.Infof("loaded environment")
	return nil
}

// resolveSecrets resolves individual secrets in the environment.
func resolveSecrets(ctx context.Context, sm secrets.SecretManager, dir string) error {
	logger := logging.FromContext(ctx)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			logger.Errorf("environment variable unexpected format: %v", e)
			continue
		}

		envName, secretRef := parts[0], parts[1]
		if strings.HasPrefix(secretRef, SecretPrefix) {
			// Short circuit if no secret manager was configured.
			if sm == nil {
				return fmt.Errorf("environment requests secrets, but no secret manager is configured")
			}

			// Remove the prefix.
			secretRef = strings.TrimPrefix(secretRef, SecretPrefix)

			// Check if the value should be written to a file.
			toFile := false
			if strings.HasSuffix(secretRef, FileSuffix) {
				toFile = true
				secretRef = strings.TrimSuffix(secretRef, FileSuffix)
			}

			logger.Infof("resolving secret value for %q (toFile=%t)", envName, toFile)

			secretVal, err := sm.GetSecretValue(ctx, secretRef)
			if err != nil {
				return fmt.Errorf("failed to resolve %q: %w", secretRef, err)
			}

			if toFile {
				if err := ensureSecureDir(dir); err != nil {
					return err
				}

				secretFileName := filenameForSecret(envName + "." + secretRef)
				secretFilePath := filepath.Join(dir, secretFileName)
				if err := ioutil.WriteFile(secretFilePath, []byte(secretVal), 0600); err != nil {
					return fmt.Errorf("failed to write secret file for %q: %w", envName, err)
				}

				logger.Infof("wrote secret file for %v", envName)
				secretVal = secretFilePath
			}

			// Replace the value of the environment variable with the either the resolved secret value
			// or the file path to the secret value saved on the filesystem.
			//
			// This takes effect for this process and any child processes only.
			// When envconfig is used to load a variable if
			//   DB_PASS was "secret://pathto/databasepassword"
			//   DB_PASS will not be the actual database password for the application to consume.
			os.Setenv(envName, secretVal)
		}
	}

	return nil
}

// filenameForSecret returns the sha1 of the secret name.
func filenameForSecret(name string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(name)))
}

// ensureSecureDir creates the parent secretsDir with minimal permissions. If
// the directory does not exist, it is created with 0700 permissions. If the
// directory exists but has broader permissions that 0700, an error is returned.
func ensureSecureDir(dir string) error {
	stat, err := os.Stat(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if secure directory %q exists: %w", dir, err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create secure directory %q: %w", dir, err)
		}
	} else {
		if stat.Mode().Perm() != os.FileMode(0700) {
			return fmt.Errorf("secure directory %q exists and is not restricted %v", dir, stat.Mode())
		}
	}
	return nil
}
