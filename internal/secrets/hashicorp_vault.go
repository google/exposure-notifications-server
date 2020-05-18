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
	"strconv"

	vaultapi "github.com/hashicorp/vault/api"
)

// Compile-time check to verify implements interface.
var _ SecretManager = (*HashiCorpVault)(nil)

type HashiCorpVault struct {
	client *vaultapi.Client
}

// NewHashiCorpVault fetches secrets from HashiCorp Vault.
func NewHashiCorpVault(ctx context.Context) (SecretManager, error) {
	client, err := vaultapi.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("secrets.NewHashiCorpVault: client: %w", err)
	}

	sm := &HashiCorpVault{
		client: client,
	}

	return sm, nil
}

// GetSecretValue implements the SecretManager interface. Secrets are specified
// as the path to the secret in Vault. Secrets are expected to have the string
// value for the secret in a key named "value".
//
// For example:
//
//     /secret/data/my-secret?version=5 #=> { "value": "dajkfl32ip2" }
//
// Note: this technically allows you to fetch dynamic secrets, but this library
// makes no attempt at renewing leases!
func (kv *HashiCorpVault) GetSecretValue(ctx context.Context, name string) (string, error) {
	secret, err := kv.client.Logical().Read(name)
	if err != nil {
		return "", fmt.Errorf("failed to access secret %v: %w", name, err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("found secret %v, but value was nil", name)
	}

	// Check if the "value" key is present.
	raw, ok := secret.Data["value"]
	if !ok {
		return "", fmt.Errorf("found secret %v, does not have 'value' key", name)
	}

	// Vault values are map[string]interface{}, so coerce to a string.
	switch typ := raw.(type) {
	case string:
		return typ, nil
	case []byte:
		return string(typ), nil
	case bool:
		return strconv.FormatBool(typ), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", typ), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typ), nil
	default:
		return "", fmt.Errorf("found secret %v, but is of unknown type %T", name, typ)
	}
}
