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

// Identifies the type of Blobestore to use
type BlobstoreType string

const (
	BlobstoreTypeNone               BlobstoreType = "NONE"
	BlobstoreTypeFilesystem         BlobstoreType = "FILESYSTEM"
	BlobstoreTypeGoogleCloudStorage BlobstoreType = "GOOGLE_CLOUD_STORAGE"
)

// Blobstore Configuration
type BlobstoreConfig struct {
	BlobstoreType BlobstoreType
}

// Blobstore defines the minimum interface for a blob storage system.
type Blobstore interface {
	// CreateObject creates or overwrites an object in the storage system.
	CreateObject(ctx context.Context, bucket, objectName string, contents []byte) error

	// DeleteObject deltes an object or does nothing if the object doesn't exist.
	DeleteObject(ctx context.Context, bucket, objectName string) error
}

// BlobstoreFor returns the blobstore for the given type, or an error if one
// does not exist.
func BlobstoreFor(ctx context.Context, typ BlobstoreType) (Blobstore, error) {
	switch typ {
	case BlobstoreTypeNone:
		return NewNoopBlobstore(ctx)
	case BlobstoreTypeFilesystem:
		return NewFilesystemStorage(ctx)
	case BlobstoreTypeGoogleCloudStorage:
		return NewGoogleCloudStorage(ctx)
	}

	return nil, fmt.Errorf("unknown storage type: %v", typ)
}
