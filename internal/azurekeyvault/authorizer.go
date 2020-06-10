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

// Package azurekeyvault provides shared functionality between the
// signing and secret clients for KeyVault
package azurekeyvault

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Authorizer only needs to be initialied once per server, treat as singleton
// guarded by a mutex.
var (
	mu   sync.Mutex
	auth autorest.Authorizer
)

// GetKeyVaultAuthorizer prepares a specifc authorizer for keyvault use
func GetKeyVaultAuthorizer() (autorest.Authorizer, error) {
	mu.Lock()
	defer mu.Unlock()

	if auth != nil {
		return auth, nil
	}

	azureEnv, err := azure.EnvironmentFromName("AzurePublicCloud")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Azure environment: %w", err)
	}

	vaultEndpoint := strings.TrimSuffix(azureEnv.KeyVaultEndpoint, "/")
	tenant := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	alternateEndpoint := &url.URL{
		Scheme: "https",
		Host:   "login.windows.net",
		Path:   path.Join(tenant, "oauth2/token"),
	}

	oauthconfig, err := adal.NewOAuthConfig(azureEnv.ActiveDirectoryEndpoint, tenant)
	if err != nil {
		return nil, fmt.Errorf("failed creating OAuth config for Azure Key Vault: %v", err)
	}
	oauthconfig.AuthorizeEndpoint = *alternateEndpoint

	token, err := adal.NewServicePrincipalToken(
		*oauthconfig,
		clientID,
		clientSecret,
		vaultEndpoint,
	)
	if err != nil {
		return nil, fmt.Errorf("failed requesting access token for Azure Key Vault: %v", err)
	}

	auth = autorest.NewBearerAuthorizer(token)

	return auth, nil
}
