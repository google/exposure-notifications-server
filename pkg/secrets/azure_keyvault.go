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

// +build azure all

package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/keyvault/keyvault"
	"github.com/google/exposure-notifications-server/internal/azurekeyvault"
)

func init() {
	RegisterManager("AZURE_KEY_VAULT", NewAzureKeyVault)
}

// Compile-time check to verify implements interface.
var _ SecretManager = (*AzureKeyVault)(nil)

// AzureKeyVault implements SecretManager.
type AzureKeyVault struct {
	client *keyvault.BaseClient
}

// NewAzureKeyVault creates a new KeyVault that can interact fetch secrets.
func NewAzureKeyVault(ctx context.Context, _ *Config) (SecretManager, error) {
	authorizer, err := azurekeyvault.GetKeyVaultAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("secrets.NewAzureKeyVault: auth: %w", err)
	}

	client := keyvault.New()
	client.Authorizer = authorizer

	sm := &AzureKeyVault{
		client: &client,
	}

	return sm, nil
}

// GetSecretValue implements the SecretManager interface. Secrets are specified
// in the format:
//
//     AZURE_KEY_VAULT_NAME/SECRET_NAME/SECRET_VERSION
//
// For example:
//
//     my-company-vault/api-key/1
//
// If the secret version is omitted, the latest version is used.
func (kv *AzureKeyVault) GetSecretValue(ctx context.Context, name string) (string, error) {
	// Extract vault, secret, and version.
	var vaultName, secretName, version string
	parts := strings.SplitN(name, "/", 3)
	switch len(parts) {
	case 0, 1:
		return "", fmt.Errorf("%v is not a valid secret ref", name)
	case 2:
		vaultName, secretName, version = parts[0], parts[1], ""
	case 3:
		vaultName, secretName, version = parts[0], parts[1], parts[2]
	}

	// Lookup in KeyVault
	vaultURL := fmt.Sprintf("https://%s.vault.azure.net", vaultName)
	result, err := kv.client.GetSecret(ctx, vaultURL, secretName, version)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}
	if result.Value == nil {
		return "", fmt.Errorf("found secret %v, but value was nil", name)
	}
	return *result.Value, nil
}
