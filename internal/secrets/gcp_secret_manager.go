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
// Allows for a different implementation to be bound within the servernv.ServeEnv
package secrets

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/exposure-notifications-server/internal/logging"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

const (
	// defaultCacheDuration is how long to cache a secret.
	defaultCacheDuration = 5 * time.Minute
)

type GCPSecretManager struct {
	client *secretmanager.Client

	cache         map[string]*item
	cacheDuration time.Duration
	cacheMutex    sync.Mutex
}

type item struct {
	value     string
	expiresAt int64
}

// Option is an option to the setup.
type Option func(*GCPSecretManager) *GCPSecretManager

// WithCacheDuration creates an Option to install a specific secret manager to use.
func WithCacheDuration(d time.Duration) Option {
	return func(s *GCPSecretManager) *GCPSecretManager {
		s.cacheDuration = d
		return s
	}
}

func NewGCPSecretManager(ctx context.Context, opts ...Option) (SecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}

	sm := &GCPSecretManager{
		client:        client,
		cache:         make(map[string]*item),
		cacheDuration: defaultCacheDuration,
	}

	for _, f := range opts {
		sm = f(sm)
	}

	return sm, nil
}

// GetSecretValue implements the SecretManager interface.
func (sm *GCPSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	logger := logging.FromContext(ctx)

	// Lock
	sm.cacheMutex.Lock()
	defer sm.cacheMutex.Unlock()

	// Lookup in cache
	if i, ok := sm.cache[name]; ok && i.expiresAt <= time.Now().UnixNano() {
		logger.Debugf("loaded secret %v from cache", name)
		return i.value, nil
	}

	// Lookup in secret manager
	plaintext, err := sm.accessSecret(ctx, name)
	if err != nil {
		return "", err
	}

	// Cache value
	sm.cache[name] = &item{
		value:     plaintext,
		expiresAt: time.Now().Add(sm.cacheDuration).UnixNano(),
	}

	return plaintext, nil
}

// accessSecret accesses the secret from secret manager.
func (sm *GCPSecretManager) accessSecret(ctx context.Context, name string) (string, error) {
	// Call the API.
	result, err := sm.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}
	return string(result.Payload.Data), nil
}
