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

//go:build vault || all

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	vaultapi "github.com/hashicorp/vault/api"
)

func init() {
	RegisterManager("HASHICORP_VAULT", NewHashiCorpVault)
}

// Compile-time check to verify implements interface.
var _ SecretManager = (*HashiCorpVault)(nil)

type HashiCorpVault struct {
	client *vaultapi.Client
}

// NewHashiCorpVault fetches secrets from HashiCorp Vault.
func NewHashiCorpVault(ctx context.Context, _ *Config) (SecretManager, error) {
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
// value for the secret in a key named "value" in the "data" key. This matches
// the schema returned by the KVv2 secrets engine:
//
//	$ vault secrets enable -version=2 kv
//	$ vault kv put my-secret value="abc123"
//
// For example:
//
//	/secret/data/my-secret #=> { "data": { "value": "dajkfl32ip2" } }
//
// Note: this technically allows you to fetch dynamic secrets, but this library
// makes no attempt at renewing leases!
func (kv *HashiCorpVault) GetSecretValue(ctx context.Context, name string) (string, error) {
	u, err := url.Parse(name)
	if err != nil {
		return "", fmt.Errorf("failed to parse name: %w", err)
	}

	name, version := u.Path, u.Query().Get("version")
	if version == "" {
		version = "1"
	}

	secret, err := kv.client.Logical().ReadWithData(name, map[string][]string{
		"version": {version},
	})
	if err != nil {
		return "", fmt.Errorf("failed to access secret: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("secret data is nil")
	}

	// Check if the "data" key is present.
	dataRaw, ok := secret.Data["data"]
	if !ok {
		return "", fmt.Errorf("missing 'data' key")
	}

	data, ok := dataRaw.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("data is not a map")
	}

	valueRaw, ok := data["value"]
	if !ok {
		return "", fmt.Errorf("missing 'value' key")
	}

	// Vault values are map[string]interface{}, so coerce to a string.
	switch typ := valueRaw.(type) {
	case string:
		return typ, nil
	case []byte:
		return string(typ), nil
	case bool:
		return strconv.FormatBool(typ), nil
	case json.Number:
		return typ.String(), nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", typ), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typ), nil
	default:
		return "", fmt.Errorf("found secret %v, but is of unknown type %T", name, typ)
	}
}
