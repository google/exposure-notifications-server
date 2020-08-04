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

// Package revision defines the internal structure of the revision token
// and utilities for marshal/unmarshal which also encrypts/decrypts the payload.
package revision

// Config represents the configuration and associated environment variables
// for handling revision tokens.
type Config struct {
	// Crypto key to use for wrapping/unwrapping the revision token cipher blocks.
	KeyID     string `env:"REVISION_TOKEN_KEY_ID"`
	AAD       string `env:"REVISION_TOKEN_AAD"` // must be base64 encoded, may come from secret://
	MinLength uint   `env:"REVISION_TOKEN_MIN_LENGTH, default=28"`
}
