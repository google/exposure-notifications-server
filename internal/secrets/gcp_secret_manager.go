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

package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// Compile-time check to verify implements interface.
var _ SecretManager = (*GCPSecretManager)(nil)

// GCPSecretManager implements SecretManager.
type GCPSecretManager struct {
	client *secretmanager.Client
}

// NewGCPSecretManager creates a new secret manager for GCP.
func NewGCPSecretManager(ctx context.Context) (SecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}

	sm := &GCPSecretManager{
		client: client,
	}

	return sm, nil
}

// GetSecretValue implements the SecretManager interface. Secret names should be
// of the format:
//
//     projects/my-project/secrets/my-secret/versions/123
func (sm *GCPSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	result, err := sm.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}
	return string(result.Payload.Data), nil
}
