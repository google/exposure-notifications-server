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

package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

// Compile-time check to verify implements interface.
var _ Blobstore = (*AzureBlobstore)(nil)

// AzureBlobstore implements the Blob interface and provides the ability
// write files to Azure Blob Storage.
type AzureBlobstore struct {
	serviceURL *azblob.ServiceURL
}

// NewAzureBlobstore creates a storage client, suitable for use with
// serverenv.ServerEnv.
func NewAzureBlobstore(ctx context.Context) (Blobstore, error) {
	accountName := os.Getenv("AZURE_STORAGE_ACCOUNT")
	if accountName == "" {
		return nil, fmt.Errorf("missing AZURE_STORAGE_ACCOUNT")
	}

	accountKey := os.Getenv("AZURE_STORAGE_ACCESS_KEY")
	if accountKey == "" {
		return nil, fmt.Errorf("missing AZURE_STORAGE_ACCESS_KEY")
	}

	primaryURLRaw := fmt.Sprintf("https://%s.blob.core.windows.net", accountName)
	primaryURL, err := url.Parse(primaryURLRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL %v: %v", primaryURLRaw, err)
	}

	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("storage.NewAzureBlobstore: %w", err)
	}
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	serviceURL := azblob.NewServiceURL(*primaryURL, p)

	return &AzureBlobstore{
		serviceURL: &serviceURL,
	}, nil
}

// CreateObject creates a new blobstore object or overwrites an existing one.
func (s *AzureBlobstore) CreateObject(ctx context.Context, container, name string, contents []byte, cacheable bool) error {
	cacheControl := "public, max-age=86400"
	if !cacheable {
		cacheControl = "no-cache, max-age=0"
	}

	blobURL := s.serviceURL.NewContainerURL(container).NewBlockBlobURL(name)
	if _, err := azblob.UploadBufferToBlockBlob(ctx, contents, blobURL, azblob.UploadToBlockBlobOptions{
		BlobHTTPHeaders: azblob.BlobHTTPHeaders{
			CacheControl: cacheControl,
		},
	}); err != nil {
		return fmt.Errorf("storage.CreateObject: %w", err)
	}
	return nil
}

// DeleteObject deletes a blobstore object, returns nil if the object was
// successfully deleted, or if the object doesn't exist.
func (s *AzureBlobstore) DeleteObject(ctx context.Context, container, name string) error {
	blobURL := s.serviceURL.NewContainerURL(container).NewBlockBlobURL(name)
	if _, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
		if terr, ok := err.(azblob.StorageError); ok && terr.ServiceCode() == azblob.ServiceCodeBlobNotFound {
			// already deleted
			return nil
		}
		return fmt.Errorf("storage.DeleteObject: %w", err)
	}
	return nil
}

// GetObject returns the contents for the given object. If the object does not
// exist, it returns ErrNotFound.
func (s *AzureBlobstore) GetObject(ctx context.Context, container, name string) ([]byte, error) {
	blobURL := s.serviceURL.NewContainerURL(container).NewBlockBlobURL(name)
	dr, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to download object: %w", err)
	}

	body := dr.Body(azblob.RetryReaderOptions{MaxRetryRequests: 5})
	defer body.Close()

	var b bytes.Buffer
	if _, err := io.Copy(&b, body); err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return b.Bytes(), nil
}
