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
)

// File System Storage implements the Blob interface and provdes the ability
// write files to File Syste Storage.
type FileSystemCloudStorage struct {
}

func NewFileSystemCloudStorage(ctx context.Context) (Blobstore, error) {
	return &FileSystemCloudStorage{}, nil
}

// CreateObject creates a new cloud storage object or overwrites an existing one.
func (gcs *FileSystemCloudStorage) CreateObject(ctx context.Context, bucket, objectName string, contents []byte) error {

	err := ioutil.WriteFile(bucket+"/"+objectName, contents, 0644)

	if err != nil {
		return fmt.Errorf("storage.Writer.Write: %w", err)
	}

	return nil
}

// DeleteObject deletes a cloud storage object, returns nil if the object was
// successfully deleted, or of the object doesn't exist.
func (gcs *FileSystemCloudStorage) DeleteObject(ctx context.Context, bucket, objectName string) error {
	err := os.Remove(bucket + "/" + objectName)
	if err != nil {
		return fmt.Errorf("storage.DeleteObject: %w", err)
	}
	return nil
}
