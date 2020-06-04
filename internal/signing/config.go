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

package signing

// KeyManagerType defines a specific key manager.
type KeyManagerType string

const (
	KeyManagerTypeAWSKMS         KeyManagerType = "AWS_KMS"
	KeyManagerTypeGoogleCloudKMS KeyManagerType = "GOOGLE_CLOUD_KMS"
	KeyManagerTypeHashiCorpVault KeyManagerType = "HASHICORP_VAULT"
	KeyManagerTypeNoop           KeyManagerType = "NOOP"
)

// Config defines configuration.
type Config struct {
	KeyManagerType KeyManagerType `env:"KEY_MANAGER,default=GOOGLE_CLOUD_KMS"`
}

// TestConfigDefaults returns a configuration populated with the default values.
// It should only be used for testing.
func TestConfigDefaults() *Config {
	return &Config{
		KeyManagerType: KeyManagerType("GOOGLE_CLOUD_KMS"),
	}
}

// TestConfigValued returns a configuration populated with values that match
// TestConfigValues() It should only be used for testing.
func TestConfigValued() *Config {
	return &Config{
		KeyManagerType: KeyManagerType("HASHICORP_VAULT"),
	}
}

// TestConfigValues returns a list of configuration that corresponds to
// TestConfigValued. It should only be used for testing.
func TestConfigValues() map[string]string {
	return map[string]string{
		"KEY_MANAGER": "HASHICORP_VAULT",
	}
}

// TestConfigOverridden returns a configuration with non-default values set. It
// should only be used for testing.
func TestConfigOverridden() *Config {
	return &Config{
		KeyManagerType: KeyManagerType("NOOP"),
	}
}
