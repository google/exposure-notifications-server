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
)

// BlobstoreType defines a specific blobstore.
type BlobstoreType string

const (
	BlobstoreTypeGoogleCloudStorage BlobstoreType = "GOOGLE_CLOUD_STORAGE"
	BlobstoreTypeFilesystem         BlobstoreType = "FILESYSTEM"
	BlobstoreTypeNoop               BlobstoreType = "NOOP"
)

// Config defines the configuration for a blobstore.
type Config struct {
	BlobstoreType BlobstoreType `envconfig:"BLOBSTORE" default:"GOOGLE_CLOUD_STORAGE"`
}

// Blobstore defines the minimum interface for a blob storage system.
type Blobstore interface {
	// CreateObject creates or overwrites an object in the storage system.
	CreateObject(ctx context.Context, bucket, objectName string, contents []byte, cacheable bool) error

	// DeleteObject deltes an object or does nothing if the object doesn't exist.
	DeleteObject(ctx context.Context, bucket, objectName string) error
}

// BlobstoreFor returns the blob store for the given type, or an error if one
// does not exist.
func BlobstoreFor(ctx context.Context, typ BlobstoreType) (Blobstore, error) {
	switch typ {
	case BlobstoreTypeGoogleCloudStorage:
		return NewGoogleCloudStorage(ctx)
	case BlobstoreTypeFilesystem:
		return NewFilesystemStorage(ctx)
	case BlobstoreTypeNoop:
		return NewNoopBlobstore(ctx)
	default:
		return nil, fmt.Errorf("unknown blob store: %v", typ)
	}
}
