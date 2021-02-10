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

package keys

// Config defines configuration.
type Config struct {
	// Type is the type of the key manager.
	Type string `env:"KEY_MANAGER, default=FILESYSTEM"`

	// CreateHSMKeys indicates than when keys are creating, HSM level
	// protection should or should not be used if available.
	// Adherence to this config setting is optional and based
	// upon the key manager implementation and underlying capabilities.
	CreateHSMKeys bool `env:"CREATE_HSM_KEYS, default=true"`

	// FilesystemRoot is the root path where keys are managed on the filesystem.
	FilesystemRoot string `env:"KEY_FILESYSTEM_ROOT"`
}
