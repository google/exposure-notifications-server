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

// BlobstoreType defines a specific blobstore.
type BlobstoreType string

const (
	BlobstoreTypeAWSS3              BlobstoreType = "AWS_S3"
	BlobstoreTypeAzureBlobStorage   BlobstoreType = "AZURE_BLOB_STORAGE"
	BlobstoreTypeFilesystem         BlobstoreType = "FILESYSTEM"
	BlobstoreTypeGoogleCloudStorage BlobstoreType = "GOOGLE_CLOUD_STORAGE"
	BlobstoreTypeMemory             BlobstoreType = "MEMORY"
	BlobstoreTypeNoop               BlobstoreType = "NOOP"
)

// Config defines the configuration for a blobstore.
type Config struct {
	BlobstoreType BlobstoreType `env:"BLOBSTORE,default=GOOGLE_CLOUD_STORAGE"`
}
