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

// Package storage is an interface over file/blob storage.
package storage

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

var ErrNotFound = fmt.Errorf("storage object not found")

const (
	ContentTypeTextPlain = "text/plain"
	ContentTypeZip       = "application/zip"
)

// Blobstore defines the minimum interface for a blob storage system.
type Blobstore interface {
	// CreateObject creates or overwrites an object in the storage system.
	// If contentType is blank, the default for the chosen storage implementation is used.
	CreateObject(ctx context.Context, parent, name string, contents []byte, cacheable bool, contentType string) error

	// DeleteObject deletes an object or does nothing if the object doesn't exist.
	DeleteObject(ctx context.Context, parent, bame string) error

	// GetObject fetches the object's contents.
	GetObject(ctx context.Context, parent, name string) ([]byte, error)
}

// BlobstoreFunc is a func that returns a blobstore or error.
type BlobstoreFunc func(context.Context, *Config) (Blobstore, error)

// blobstores is the list of registered blobstores.
var blobstores = make(map[string]BlobstoreFunc)
var blobstoresLock sync.RWMutex

// RegisterBlobstore registers a new blobstore with the given name. If a blobstore
// is already registered with the given name, it panics. Blobstores are usually
// registered via an init function.
func RegisterBlobstore(name string, fn BlobstoreFunc) {
	blobstoresLock.Lock()
	defer blobstoresLock.Unlock()

	if _, ok := blobstores[name]; ok {
		panic(fmt.Sprintf("blobstore %q is already registered", name))
	}
	blobstores[name] = fn
}

// RegisteredBlobstores returns the list of the names of the registered
// blobstores.
func RegisteredBlobstores() []string {
	blobstoresLock.RLock()
	defer blobstoresLock.RUnlock()

	list := make([]string, 0, len(blobstores))
	for k := range blobstores {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

// BlobstoreFor returns the blobstore with the given name, or an error if one
// does not exist.
func BlobstoreFor(ctx context.Context, cfg *Config) (Blobstore, error) {
	blobstoresLock.RLock()
	defer blobstoresLock.RUnlock()

	name := cfg.Type
	fn, ok := blobstores[name]
	if !ok {
		return nil, fmt.Errorf("unknown or uncompiled blobstore %q", name)
	}
	return fn(ctx, cfg)
}
