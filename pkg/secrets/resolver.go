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

package secrets

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/sethvargo/go-envconfig"
)

const (
	// SecretPrefix is the prefix, that if the value of an env var starts with
	// will be resolved through the configured secret store.
	SecretPrefix = "secret://"

	// FileSuffix is the suffix to use, if this secret path should be written to a file.
	// only interpreted on environment variable values that start w/ secret://
	FileSuffix = "?target=file"
)

// Resolver returns a function that fetches secrets from the secret manager. If
// the provided secret manager is nil, the function is nil, Otherwise, it looks
// for values prefixed with secret:// and resolves them as secrets. For slice
// functions, values separated by commas are processed as individual secrets.
func Resolver(sm SecretManager, config *Config) envconfig.MutatorFunc {
	if sm == nil {
		return nil
	}

	resolver := &secretResolver{
		sm:  sm,
		dir: config.SecretsDir,
	}

	return func(ctx context.Context, key, value string) (string, error) {
		vals := strings.Split(value, ",")
		resolved := make([]string, len(vals))

		for i, val := range vals {
			s, err := resolver.resolve(ctx, key, val)
			if err != nil {
				return "", err
			}
			resolved[i] = s
		}

		return strings.Join(resolved, ","), nil
	}
}

type secretResolver struct {
	sm  SecretManager
	dir string
}

// resolve resolves an individual secretRef using the provided secret manager
// and configuration.
func (r *secretResolver) resolve(ctx context.Context, envName, secretRef string) (string, error) {
	logger := logging.FromContext(ctx)

	if !strings.HasPrefix(secretRef, SecretPrefix) {
		return secretRef, nil
	}

	if r.sm == nil {
		return "", fmt.Errorf("env requested secrets, but no secret manager is configured")
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

	secretVal, err := r.sm.GetSecretValue(ctx, secretRef)
	if err != nil {
		return "", fmt.Errorf("failed to resolve %q: %w", secretRef, err)
	}

	if toFile {
		if err := r.ensureSecureDir(); err != nil {
			return "", err
		}

		secretFileName := r.filenameForSecret(envName + "." + secretRef)
		secretFilePath := filepath.Join(r.dir, secretFileName)
		if err := os.WriteFile(secretFilePath, []byte(secretVal), 0o600); err != nil {
			return "", fmt.Errorf("failed to write secret file for %q: %w", envName, err)
		}

		secretVal = secretFilePath
	}

	return secretVal, nil
}

// filenameForSecret returns the sha1 of the secret name.
func (r *secretResolver) filenameForSecret(name string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(name))) //nolint:gosec
}

// ensureSecureDir creates the parent secretsDir with minimal permissions. If
// the directory does not exist, it is created with 0700 permissions. If the
// directory exists but has broader permissions that 0700, an error is returned.
func (r *secretResolver) ensureSecureDir() error {
	stat, err := os.Stat(r.dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if secure directory %q exists: %w", r.dir, err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(r.dir, 0o700); err != nil {
			return fmt.Errorf("failed to create secure directory %q: %w", r.dir, err)
		}
	} else {
		if stat.Mode().Perm() != os.FileMode(0o700) {
			return fmt.Errorf("secure directory %q exists and is not restricted %v", r.dir, stat.Mode())
		}
	}
	return nil
}
