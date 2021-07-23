// Copyright 2020 the Exposure Notifications Server authors
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

package authorizedapp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
)

// Compile-time check to assert implementation.
var _ Provider = (*MemoryProvider)(nil)

// MemoryProvider is an Provider that stores values in-memory. It is primarily
// used for testing.
type MemoryProvider struct {
	lock sync.RWMutex
	data map[string]*model.AuthorizedApp
}

// NewMemoryProvider creates a new Provider that is in memory.
func NewMemoryProvider(_ context.Context, _ *Config) (Provider, error) {
	provider := &MemoryProvider{
		data: make(map[string]*model.AuthorizedApp),
	}
	return provider, nil
}

// AppConfig returns the config for the given app package name.
func (p *MemoryProvider) AppConfig(_ context.Context, name string) (*model.AuthorizedApp, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	// Match case-insensitivy of the database.
	name = strings.ToLower(name)

	val, ok := p.data[name]
	if !ok {
		return nil, ErrAppNotFound
	}
	return val, nil
}

// Add inserts the app. It returns an error if the app already exists.
func (p *MemoryProvider) Add(_ context.Context, app *model.AuthorizedApp) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	name := strings.ToLower(app.AppPackageName)

	if _, ok := p.data[name]; ok {
		return fmt.Errorf("%v already exists", name)
	}

	p.data[name] = app
	return nil
}
