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
	"strings"
	"sync"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Authorizer provides a mutex for working with the Key Vault auhtorizer
type Authorizer struct {
	lock sync.Mutex
	auth autorest.Authorizer
}

// keyvaultAuthorizer is a cached authorizer.
var keyvaultAuthorizer *Authorizer

// GetKeyVaultAuthorizer prepares a specifc authorizer for keyvault use
func GetKeyVaultAuthorizer() (autorest.Authorizer, error) {
	keyvaultAuthorizer.lock.Lock()
	defer keyvaultAuthorizer.lock.Unlock()

	if keyvaultAuthorizer.auth != nil {
		return keyvaultAuthorizer.auth, nil
	}

	var a autorest.Authorizer
	azureEnv, err := azure.EnvironmentFromName("AzurePublicCloud")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Azure environment: %w", err)
	}
	vaultEndpoint := strings.TrimSuffix(azureEnv.KeyVaultEndpoint, "/")
	tenant := os.Getenv("AZURE_TENANT_ID")
	clientID := os.Getenv("AZURE_CLIENT_ID")
	clientSecret := os.Getenv("AZURE_CLIENT_SECRET")

	alternateEndpoint, err := url.Parse(
		"https://login.windows.net/" + tenant + "/oauth2/token",
	)
	if err != nil {
		return a, fmt.Errorf("failed parsing Azure Key Vault endpoint: %v", err)
	}

	oauthconfig, err := adal.NewOAuthConfig(azureEnv.ActiveDirectoryEndpoint, tenant)
	if err != nil {
		return a, fmt.Errorf("failed creating OAuth config for Azure Key Vault: %v", err)
	}
	oauthconfig.AuthorizeEndpoint = *alternateEndpoint

	token, err := adal.NewServicePrincipalToken(
		*oauthconfig,
		clientID,
		clientSecret,
		vaultEndpoint,
	)
	if err != nil {
		return a, fmt.Errorf("failed requesting access token for Azure Key Vault: %v", err)
	}

	keyvaultAuthorizer.auth = autorest.NewBearerAuthorizer(token)

	return keyvaultAuthorizer.auth, err
}
