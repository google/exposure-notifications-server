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

// Package secrets defines a minimum abstract interface for a secret manager.
// Allows for a different implementation to be bound within the ServeEnv.
//
// Although exported, this package is non intended for general consumption. It
// is a shared dependency between multiple exposure notifications projects. We
// cannot guarantee that there won't be breaking changes in the future.
package secrets

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// SecretManager defines the minimum shared functionality for a secret manager
// used by this application.
type SecretManager interface {
	GetSecretValue(ctx context.Context, name string) (string, error)
}

// SecretManagerFunc is a func that returns a secret manager or error.
type SecretManagerFunc func(context.Context, *Config) (SecretManager, error)

// managers is the list of registered secret managers.
var (
	managers     = make(map[string]SecretManagerFunc)
	managersLock sync.RWMutex
)

// RegisterManager registers a new secret manager with the given name. If a
// manager is already registered with the given name, it panics. Managers are
// usually registered via an init function.
func RegisterManager(name string, fn SecretManagerFunc) {
	managersLock.Lock()
	defer managersLock.Unlock()

	if _, ok := managers[name]; ok {
		panic(fmt.Sprintf("secret manager %q is already registered", name))
	}
	managers[name] = fn
}

// RegisteredManagers returns the list of the names of the registered secret
// managers.
func RegisteredManagers() []string {
	managersLock.RLock()
	defer managersLock.RUnlock()

	list := make([]string, 0, len(managers))
	for k := range managers {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

// SecretManagerFor returns the secret manager with the given name, or an error
// if one does not exist.
func SecretManagerFor(ctx context.Context, cfg *Config) (SecretManager, error) {
	managersLock.RLock()
	defer managersLock.RUnlock()

	name := cfg.Type
	fn, ok := managers[name]
	if !ok {
		return nil, fmt.Errorf("unknown or uncompiled secret manager %q", name)
	}
	return fn(ctx, cfg)
}
