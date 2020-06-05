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
	KeyManagerTypeAzureKeyVault  KeyManagerType = "AZURE_KEY_VAULT"
	KeyManagerTypeGoogleCloudKMS KeyManagerType = "GOOGLE_CLOUD_KMS"
	KeyManagerTypeHashiCorpVault KeyManagerType = "HASHICORP_VAULT"
	KeyManagerTypeNoop           KeyManagerType = "NOOP"
)

// Config defines configuration.
type Config struct {
	KeyManagerType KeyManagerType `env:"KEY_MANAGER,default=GOOGLE_CLOUD_KMS"`
}
