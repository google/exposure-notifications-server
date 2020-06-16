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
	"context"
	"path"
	"sync"
)

// Compile-time check to verify implements interface.
var _ Blobstore = (*Memory)(nil)

// Memory implements Blobstore and provides the ability write files to
// memory.
type Memory struct {
	lock sync.Mutex
	data map[string][]byte
}

// NewMemory creates a Blobstore that writes data in memory.
func NewMemory(_ context.Context) (Blobstore, error) {
	return &Memory{
		data: make(map[string][]byte),
	}, nil
}

// CreateObject creates a new object.
func (s *Memory) CreateObject(_ context.Context, folder, filename string, contents []byte, cacheable bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	pth := path.Join(folder, filename)
	s.data[pth] = contents
	return nil
}

// DeleteObject deletes an object. It returns nil if the object was deleted or
// if the object no longer exists.
func (s *Memory) DeleteObject(_ context.Context, folder, filename string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	pth := path.Join(folder, filename)
	delete(s.data, pth)
	return nil
}

// GetObject returns the contents for the given object. If the object does not
// exist, it returns ErrNotFound.
func (s *Memory) GetObject(_ context.Context, folder, filename string) ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	pth := path.Join(folder, filename)
	v, ok := s.data[pth]
	if !ok {
		return nil, ErrNotFound
	}
	return v, nil
}
