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
var _ Blobstore = (*Noop)(nil)

// Noop is a blobstore that does nothing.
type Noop struct{}

func NewNoop(ctx context.Context) (Blobstore, error) {
	return &Noop{}, nil
}

func (s *Noop) CreateObject(_ context.Context, _, _ string, _ []byte, _ bool) error {
	return nil
}

func (s *Noop) DeleteObject(_ context.Context, _, _ string) error {
	return nil
}

func (s *Noop) GetObject(_ context.Context, _, _ string) ([]byte, error) {
	return nil, nil
}
