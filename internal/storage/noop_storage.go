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

import "context"

// Compile-time check to verify implements.
var _ Blobstore = (*NoopBlobstore)(nil)

// NoopBlobstore is a blobstore that does nothing.
type NoopBlobstore struct{}

func NewNoopBlobstore(ctx context.Context) (Blobstore, error) {
	return &NoopBlobstore{}, nil
}

func (s *NoopBlobstore) CreateObject(ctx context.Context, folder, filename string, contents []byte, cacheable bool) error {
	return nil
}

func (s *NoopBlobstore) DeleteObject(ctx context.Context, folder, filename string) error {
	return nil
}
