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

// Package azurekeyvault provides shared functionality between the
// signing and secret clients for KeyVault
package azurekeyvault

import (
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest"
)

// Authorizer only needs to be initialied once per server, treat as singleton
// guarded by a mutex.
var (
	mu         sync.Mutex
	authorizer autorest.Authorizer
)

// GetKeyVaultAuthorizer prepares a specifc authorizer for keyvault use.
func GetKeyVaultAuthorizer() (autorest.Authorizer, error) {
	mu.Lock()
	defer mu.Unlock()

	if authorizer != nil {
		return authorizer, nil
	}

	kvAuth, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create KeyVault authorizer: %w", err)
	}

	authorizer = kvAuth
	return authorizer, nil
}
