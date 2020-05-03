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
	"time"

	"cloud.google.com/go/storage"
)

// CreateObject creates a new cloud storage object
func CreateObject(ctx context.Context, bucket, objectName string, contents []byte) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	wc := client.Bucket(bucket).Object(objectName).NewWriter(ctx)
	if _, err = wc.Write(contents); err != nil {
		return fmt.Errorf("storage.Writer.Write: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("storage.Writer.Close: %v", err)
	}
	return nil
}

// DeleteObject deletes a cloud storage object
func DeleteObject(ctx context.Context, bucket, objectName string) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	if err := client.Bucket(bucket).Object(objectName).Delete(ctx); err != nil {
		if err == storage.ErrObjectNotExist {
			// Object doesn't exist; presumably already deleted.
			return nil
		}
		return fmt.Errorf("storage.DeleteObject: %v", err)
	}
	return nil
}
