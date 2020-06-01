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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Compile-time check to verify implements interface.
var _ Blobstore = (*AWSS3)(nil)

// AWSS3 implements the Blob interface and provides the ability
// write files to Google Cloud Storage.
type AWSS3 struct {
	svc *s3.S3
}

// NewAWSS3 creates a Google Cloud Storage Client, suitable
// for use with serverenv.ServerEnv.
func NewAWSS3(ctx context.Context) (Blobstore, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	svc := s3.New(sess)

	return &AWSS3{
		svc: svc,
	}, nil
}

// CreateObject creates a new S3 object or overwrites an existing one.
func (s *AWSS3) CreateObject(ctx context.Context, bucket, key string, contents []byte, cacheable bool) error {
	cacheControl := "public, max-age=86400"
	if !cacheable {
		cacheControl = "no-cache, max-age=0"
	}

	if _, err := s.svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:       aws.String(bucket),
		Key:          aws.String(key),
		CacheControl: aws.String(cacheControl),
		Body:         bytes.NewReader(contents),
	}); err != nil {
		return fmt.Errorf("storage.CreateObject: %w", err)
	}
	return nil
}

// DeleteObject deletes an S3 object, returns nil if the object was successfully
// deleted, or of the object doesn't exist.
func (s *AWSS3) DeleteObject(ctx context.Context, bucket, key string) error {
	if _, err := s.svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		return fmt.Errorf("storage.DeleteObject: %w", err)
	}
	return nil
}
