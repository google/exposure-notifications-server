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
	"context"
	"fmt"
	"time"

	"github.com/google/exposure-notifications-server/internal/android"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/model/apiconfig"
)

var ValidateAttestation = android.ValidateAttestation

// VerifyRegions checks the request regions against the regions allowed by
// the configuration for the application.
func VerifyRegions(cfg *apiconfig.APIConfig, data model.Publish) error {
	if cfg == nil {
		return fmt.Errorf("no allowed regions configured")
	}

	if cfg.AllowAllRegions {
		return nil
	}

	for _, r := range data.Regions {
		if v, ok := cfg.AllowedRegions[r]; !ok || !v {
			return fmt.Errorf("application '%v' tried to write unauthorized region: '%v'", cfg.AppPackageName, r)
		}
	}

	// no error - application didn't try to write for regions that it isn't allowed
	return nil
}

// VerifySafetyNet verifies the SafetyNet device attestation against the allowed configuration for the application.
func VerifySafetyNet(ctx context.Context, requestTime time.Time, cfg *apiconfig.APIConfig, data model.Publish) error {
	logger := logging.FromContext(ctx)

	if cfg == nil {
		logger.Errorf("safetynet enabled, but no config for application: %v", data.AppPackageName)
		// TODO(mikehelmick): Should this be a default configuration?
		return fmt.Errorf("cannot enforce safetynet, no application config")
	}

	opts := cfg.VerifyOpts(requestTime, android.NewNonce(&data))
	if err := ValidateAttestation(ctx, data.Verification, opts); err != nil {
		if cfg.BypassSafetyNet {
			logger.Errorf("bypassing safetynet verification for: '%v'", data.AppPackageName)
			return nil
		}

		return fmt.Errorf("android.ValidateAttestation: %w", err)
	}

	return nil
}
