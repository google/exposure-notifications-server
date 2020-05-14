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

var (
	// cacheDuration is how long to cache a secret.
	cacheDuration = 5 * time.Minute
)

type GCPSecretManager struct {
	client *secretmanager.Client

	cache      map[string]*item
	cacheMutex sync.RWMutex
}

type item struct {
	value     string
	expiresAt int64
}

func NewGCPSecretManager(ctx context.Context) (SecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}

	return &GCPSecretManager{
		client: client,
		cache:  make(map[string]*item),
	}, nil
}

func (sm *GCPSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	logger := logging.FromContext(ctx)

	// Check cache.
	sm.cacheMutex.RLock()
	if i, ok := sm.cache[name]; ok && i.expiresAt <= time.Now().UnixNano() {
		sm.cacheMutex.RUnlock()
		logger.Debugf("found secret in cache: %v", name)
		return i.value, nil
	}
	sm.cacheMutex.RUnlock()

	// Build the request.
	accessRequest := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	// Call the API.
	result, err := sm.client.AccessSecretVersion(ctx, accessRequest)
	if err != nil {
		return "", fmt.Errorf("failed to access secret version for %v: %w", name, err)
	}
	logger.Infof("loaded secret value for %v", name)
	plaintext := string(result.Payload.Data)

	// Cache the value.
	sm.cacheMutex.Lock()
	sm.cache[name] = &item{
		value:     plaintext,
		expiresAt: time.Now().Add(cacheDuration).UnixNano(),
	}
	sm.cacheMutex.Unlock()

	return plaintext, nil
}
