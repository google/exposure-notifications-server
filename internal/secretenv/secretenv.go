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
package secretenv

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/secrets"
	"github.com/kelseyhightower/envconfig"
)

// SecretManager defines the minimum shared functionality for a secret manager
// used by this application.
//type SecretManager interface {
//	GetSecretValue(ctx context.Context, name string) (string, error)
//}

const (
	// SecretProto is the prefix, that if the value of an env var starts with
	// will be resolved through the configured secret store.
	SecretProto = "secret://"
)

type Config struct {
	SecretFileDir string `envconfig:"SECRETS_DIR" default:"/var/run/secrets"`
}

// Process uses `envconfig.Process` first, and then crawls the structure again
// looking for values that start with "secret://" and resolving those
// values through the configured `secrets.SecretManager`
func Process(ctx context.Context, prefix string, spec interface{}, sm secrets.SecretManager) error {
	logger := logging.FromContext(ctx)
	// Always load secret config first
	config := &Config{}
	err := envconfig.Process("sercretenv", config)
	if err != nil {
		return fmt.Errorf("error loading secretenv.Config: %w", err)
	}

	err = envconfig.Process(prefix, spec)
	if err != nil {
		return fmt.Errorf("error loading environment variables: %w", err)
	}
	logger.Infof("Loaded environment: %v", spec)

	err = config.processSecrets(ctx, spec, sm)
	if err != nil {
		return err
	}

	return nil
}

// filenameForSecret returns the full filepath for the given secret. The
// filename is the sha1 of the secret name, and the path is secretsDir.
func (c *Config) filenameForSecret(name string) string {
	digest := fmt.Sprintf("%x", sha1.Sum([]byte(name)))
	return filepath.Join(c.SecretFileDir, digest)
}

func (c *Config) ensureSecretDirExists() error {
	// Create the parent secretsDir with minimal permissions. If the directory
	// does not exist, it is created with 0700 permissions. If the directory
	// exists but has broader permissions that 0700, an error is returned.
	stat, err := os.Stat(c.SecretFileDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to check if secretsDir exists: %w", err)
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(c.SecretFileDir, 0700); err != nil {
			return fmt.Errorf("failed to create secretsDir: %w", err)
		}
	} else {
		if stat.Mode().Perm() != os.FileMode(0700) {
			return fmt.Errorf("secretsDir exists and is not restricted %v", stat.Mode())
		}
	}
	return nil
}

// Even if sm is nil, look for fields w/ the secret tag.
func (c *Config) processSecrets(ctx context.Context, spec interface{}, sm secrets.SecretManager) error {
	logger := logging.FromContext(ctx)

	s := reflect.ValueOf(spec).Elem()
	typeOfSpec := s.Type()

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fType := typeOfSpec.Field(i)
		if !f.CanSet() || isTrue(fType.Tag.Get("ignored")) {
			continue
		}

		if f.Kind() == reflect.Struct {
			c.processSecrets(ctx, f.Addr().Interface(), sm)
			continue
		}

		if f.Kind() == reflect.String {
			curValue := f.String()
			if strings.Index(curValue, SecretProto) == 0 {
				if sm == nil {
					return fmt.Errorf("environment contains secret values, but there is no secret manager configured")
				}

				secretName := strings.Join(strings.Split(curValue, SecretProto), "")
				secretValue, err := sm.GetSecretValue(ctx, secretName)
				if err != nil {
					return fmt.Errorf("GetSecretValue: %v, error: %w", secretName, err)
				}

				if isTrue(fType.Tag.Get("secretfile")) {
					if err := c.ensureSecretDirExists(); err != nil {
						return err
					}
					envVar := fType.Tag.Get("envconfig")
					if envVar == "" {
						envVar = typeOfSpec.PkgPath() + "." + typeOfSpec.Name() + "." + fType.Name
					}

					secretPath := c.filenameForSecret(envVar)
					if err := ioutil.WriteFile(secretPath, []byte(secretValue), 0600); err != nil {
						return fmt.Errorf("failed to write secret %q to file: %w", envVar, err)
					}
					logger.Infof("wrote secret file for %v", envVar)
					f.SetString(secretPath)
				} else {
					f.SetString(secretValue)
				}
			}
		}
	}

	return nil
}

func isTrue(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
