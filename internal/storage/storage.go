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

// Package storage is an interface over file/blob storage
package storage

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/logging"
)

// Identifies the type of Blobestore to use
type BlobstoreType string

const (
	None       BlobstoreType = "NONE"
	Gcs        BlobstoreType = "GCS"
	Filesystem BlobstoreType = "FILESYSTEM"
)

// Blobstore Configuration
type BlobstoreConfig struct {
	Factory BlobstoreFactory
}

// Blobstore defines the minimum interface for a blob storage system.
type Blobstore interface {
	// CreateObject creates or overwrites an object in the storage system.
	CreateObject(ctx context.Context, bucket, objectName string, contents []byte) error

	// DeleteObject deltes an object or does nothing if the object doesn't exist.
	DeleteObject(ctx context.Context, bucket, objectName string) error
}

// Creates a new Blobstore using the provided context
type BlobstoreFactory func(ctx context.Context) (Blobstore, error)

// Blobstore that does nothing.
type NoopBlobstore struct{}

func NewNoopBlobstore(ctx context.Context) (Blobstore, error) {
	return &NoopBlobstore{}, nil
}

// No op.
func (s *NoopBlobstore) CreateObject(ctx context.Context, folder, filename string, contents []byte) error {
	return nil
}

// No op.
func (s *NoopBlobstore) DeleteObject(ctx context.Context, folder, filename string) error {
	return nil
}

// Creates a new BlobstoreFactory based on the provided BlobstoreType.
func NewBlobstoreFactory(blobstoreType BlobstoreType) BlobstoreFactory {
	return func(ctx context.Context) (Blobstore, error) {
		logger := logging.FromContext(ctx)
		logger.Infof("BlobstoreType is set up to %v", blobstoreType)

		switch blobstoreType {
		case Gcs:
			return NewGoogleCloudStorage(ctx)
		case Filesystem:
			return NewFilesystemStorage(ctx)
		case None:
			return NewNoopBlobstore(ctx)
		default:
			return nil, fmt.Errorf("Unknown BlobstoreType: %v", blobstoreType)
		}
	}

}
