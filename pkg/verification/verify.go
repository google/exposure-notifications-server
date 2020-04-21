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

package verification

import (
	"cambio/pkg/logging"
	"context"
)

func VerifySafetyNet(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Error("SAFETYNET VERIFICATION NOT IMPLEMENTED.")
	// TODO - implement the verification of the payload
	return nil
}
