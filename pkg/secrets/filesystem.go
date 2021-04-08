// Copyright 2021 Google LLC
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
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() {
	RegisterManager("FILESYSTEM", NewFilesystem)
}

// Compile-time check to verify implements interface.
var _ SecretVersionManager = (*Filesystem)(nil)

// Filesystem is a local filesystem based secret manager, primarily used for
// local development and testing.
type Filesystem struct {
	root string
	mu   sync.Mutex
}

// NewFilesystem creates a new filesystem-based secret manager.
func NewFilesystem(ctx context.Context, cfg *Config) (SecretManager, error) {
	root := cfg.FilesystemRoot
	if root != "" {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return nil, err
		}
	}

	return &Filesystem{
		root: root,
	}, nil
}

// GetSecretValue returns the secret if it exists, otherwise an error.
func (sm *Filesystem) GetSecretValue(ctx context.Context, name string) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	pth := filepath.Join(sm.root, name)
	b, err := os.ReadFile(pth)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(b), nil
}

// CreateSecretVersion creates a new secret version on the given parent with the
// provided data. It returns a reference to the created version.
func (sm *Filesystem) CreateSecretVersion(ctx context.Context, parent string, data []byte) (string, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	version := strconv.FormatInt(time.Now().UnixNano(), 10)
	pth := filepath.Join(sm.root, parent, version)
	if err := os.WriteFile(pth, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to create secret file: %w", err)
	}
	return strings.TrimPrefix(pth, sm.root), nil
}

// DestroySecretVersion destroys the secret version with the given name. If the
// version does not exist, no action is taken.
func (sm *Filesystem) DestroySecretVersion(ctx context.Context, name string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	pth := filepath.Join(sm.root, name)
	if err := os.Remove(pth); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to destroy secret version: %w", err)
	}
	return nil
}
