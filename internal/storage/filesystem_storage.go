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

// Package storage is an interface over Google Cloud Storage.
package storage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func init() {
	RegisterBlobstore("FILESYSTEM", NewFilesystemStorage)
}

// Compile-time check to verify implements interface.
var _ Blobstore = (*FilesystemStorage)(nil)

// FilesystemStorage implements Blobstore and provides the ability
// write files to the filesystem.
type FilesystemStorage struct{}

// NewFilesystemStorage creates a Blobsstore compatible storage for the
// filesystem.
func NewFilesystemStorage(ctx context.Context, _ *Config) (Blobstore, error) {
	return &FilesystemStorage{}, nil
}

// CreateObject creates a new object on the filesystem or overwrites an existing
// one.
// contentType is ignored for this storage implementation.
func (s *FilesystemStorage) CreateObject(ctx context.Context, folder, filename string, contents []byte, cacheable bool, contentType string) error {
	pth := filepath.Join(folder, filename)
	if err := ioutil.WriteFile(pth, contents, 0644); err != nil {
		return fmt.Errorf("failed to create object: %w", err)
	}
	return nil
}

// DeleteObject deletes an object from the filesystem. It returns nil if the
// object was deleted or if the object no longer exists.
func (s *FilesystemStorage) DeleteObject(ctx context.Context, folder, filename string) error {
	pth := filepath.Join(folder, filename)
	if err := os.Remove(pth); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// GetObject returns the contents for the given object. If the object does not
// exist, it returns ErrNotFound.
func (s *FilesystemStorage) GetObject(ctx context.Context, folder, filename string) ([]byte, error) {
	pth := filepath.Join(folder, filename)
	b, err := ioutil.ReadFile(pth)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return b, nil
}
