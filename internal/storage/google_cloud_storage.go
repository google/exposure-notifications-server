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
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
)

// Compile-time check to verify implements interface.
var _ Blobstore = (*GoogleCloudStorage)(nil)

// GoogleCloudStorage implements the Blob interface and provides the ability
// write files to Google Cloud Storage.
type GoogleCloudStorage struct {
	client *storage.Client
}

// NewGoogleCloudStorage creates a Google Cloud Storage Client, suitable
// for use with serverenv.ServerEnv
func NewGoogleCloudStorage(ctx context.Context) (Blobstore, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage.NewClient: %w", err)
	}
	return &GoogleCloudStorage{client}, nil
}

// CreateObject creates a new cloud storage object or overwrites an existing one.
func (gcs *GoogleCloudStorage) CreateObject(ctx context.Context, bucket, objectName string, contents []byte, cacheable bool) error {
	wc := gcs.client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	if !cacheable {
		wc.Metadata = map[string]string{
			"Cache-Control": "no-cache, max-age=0",
		}
	}
	if _, err := wc.Write(contents); err != nil {
		return fmt.Errorf("storage.Writer.Write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("storage.Writer.Close: %w", err)
	}
	return nil
}

// DeleteObject deletes a cloud storage object, returns nil if the object was
// successfully deleted, or of the object doesn't exist.
func (gcs *GoogleCloudStorage) DeleteObject(ctx context.Context, bucket, objectName string) error {
	if err := gcs.client.Bucket(bucket).Object(objectName).Delete(ctx); err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			// Object doesn't exist; presumably already deleted.
			return nil
		}
		return fmt.Errorf("storage.DeleteObject: %w", err)
	}
	return nil
}
