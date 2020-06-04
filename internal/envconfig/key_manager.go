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

package envconfig

import (
	"github.com/google/exposure-notifications-server/internal/signing"
)

// KeyManagerMutatorFunc returns a function that currently does nothing. It
// could be extended to resolve encrypted values, for example.
func KeyManagerMutatorFunc(km signing.KeyManager, kmConfig *signing.Config) MutatorFunc {
	if km == nil {
		return nil
	}

	// TODO: maybe support encrypted resolutions.
	return nil
}
