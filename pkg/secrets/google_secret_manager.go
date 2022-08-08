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

//go:build google || all

package secrets

import (
	"context"
	"fmt"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func init() {
	RegisterManager("GOOGLE_SECRET_MANAGER", NewGoogleSecretManager)
}

// Compile-time check to verify implements interface.
var _ SecretVersionManager = (*GoogleSecretManager)(nil)

// GoogleSecretManager implements SecretManager.
type GoogleSecretManager struct {
	client *secretmanager.Client
}

// NewGoogleSecretManager creates a new secret manager for GCP.
func NewGoogleSecretManager(ctx context.Context, _ *Config) (SecretManager, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secretmanager.NewClient: %w", err)
	}

	sm := &GoogleSecretManager{
		client: client,
	}

	return sm, nil
}

// GetSecretValue implements the SecretManager interface. Secret names should be
// of the format:
//
//	projects/my-project/secrets/my-secret/versions/123
func (sm *GoogleSecretManager) GetSecretValue(ctx context.Context, name string) (string, error) {
	result, err := sm.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}
	return string(result.Payload.Data), nil
}

// CreateSecretVersion creates a new secret version on the given parent with the
// provided data. It returns a reference to the created version.
func (sm *GoogleSecretManager) CreateSecretVersion(ctx context.Context, parent string, data []byte) (string, error) {
	version, err := sm.client.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create secret version: %w", err)
	}
	return version.GetName(), nil
}

// DestroySecretVersion destroys the secret version with the given name. If the
// version does not exist, no action is taken.
func (sm *GoogleSecretManager) DestroySecretVersion(ctx context.Context, name string) error {
	if _, err := sm.client.DestroySecretVersion(ctx, &secretmanagerpb.DestroySecretVersionRequest{
		Name: name,
	}); err != nil {
		if grpcstatus.Code(err) == grpccodes.NotFound {
			return nil
		}

		return fmt.Errorf("failed to destroy secret version: %w", err)
	}
	return nil
}
