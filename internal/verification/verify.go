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
	"os"
	"time"

	"github.com/googlepartners/exposure-notifications/internal/android"
	"github.com/googlepartners/exposure-notifications/internal/logging"
	"github.com/googlepartners/exposure-notifications/internal/model"
)

var (
	// Is safetynet being enforced on this server.
	// TODO(mikehelmick): Remove after client verification.
	enforce = true
)

func init() {
	disableSN := os.Getenv("DISABLE_SAFETYNET")
	if disableSN != "" {
		logger := logging.FromContext(context.Background())
		logger.Errorf("SafetyNet verification disabled, to enable unset the DISABLE_SAFETYNET environment variable")
		enforce = false
	}
}

func VerifyRegions(cfg *model.APIConfig, data model.Publish) error {
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

func VerifySafetyNet(ctx context.Context, requestTime time.Time, cfg *model.APIConfig, data model.Publish) error {
	logger := logging.FromContext(ctx)
	if !enforce {
		logger.Error("skipping safetynet verification, disabled by override")
		return nil
	}

	if cfg == nil {
		logger.Errorf("safetynet enabled, but no config for application: %v", data.AppPackageName)
		// TODO(mikehelmick): Should this be a default configuration?
		return fmt.Errorf("cannot enforce safetynet, no application config")
	}

	opts := cfg.VerifyOpts(requestTime.UTC())
	err := android.ValidateAttestation(ctx, data.Verification, opts)
	if err != nil {
		if cfg.BypassSafetynet {
			logger.Errorf("safetynet failed, but bypass enabled for app: '%v', failure: %v", data.AppPackageName, err)
			return nil
		}
		return fmt.Errorf("android.ValidateAttestation: %v", err)
	}

	return nil
}
