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

package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type JSONExpander struct {
	sm SecretManager
}

// WrapJSONExpander wraps an existing SecretManager with json-expansion logic.
func WrapJSONExpander(ctx context.Context, sm SecretManager) (SecretManager, error) {
	return &JSONExpander{
		sm: sm,
	}, nil
}

// GetSecretValue implements the SecretManager interface, but allows for json-expansion
// of the secret-value. If the secret name contains a period, the secret value is expected
// to be json. The secret name is assumed to come before the period, while the map-key is
// expected to follow.
//
// For example:
// If a secret with a name of "psqlcreds" has a value of `{"username":"gandalf", "password":"abc"}`
// When GetSecretValue(ctx, "psqlcreds") is called, the raw json value will be returned.
// When GetSecretValue(ctx, "psql.username") is called, only "gandalf" (without quotes) will be returned.
func (sm *JSONExpander) GetSecretValue(ctx context.Context, name string) (string, error) {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return sm.sm.GetSecretValue(ctx, name)
	}
	secretName := parts[0]
	jsonExpansionPath := parts[1:]

	// Get the value from the embdedded secret-manager using what comes before the period.
	smValue, err := sm.sm.GetSecretValue(ctx, secretName)
	if err != nil {
		return "", err
	}

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(smValue), &m); err != nil {
		return "", err
	}

	var stringValue string
	for _, p := range jsonExpansionPath {
		v, ok := m[p]
		if !ok {
			return "", fmt.Errorf("missing key %v", p)
		}
		stringValue, ok = v.(string)
		if !ok {
			m, ok = v.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("not a string or a nested field: %v", p)
			}
		}
	}

	return stringValue, nil
}
