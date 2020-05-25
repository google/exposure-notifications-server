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
	authorizedapp "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"

	"github.com/google/exposure-notifications-server/internal/ios"
)

var (
	androidValidateAttestation = android.ValidateAttestation
	iosValidateDeviceToken     = ios.ValidateDeviceToken
)

// VerifyRegions checks the request regions against the regions allowed by
// the configuration for the application.
func VerifyRegions(cfg *authorizedapp.AuthorizedApp, data *publishmodel.Publish) error {
	if cfg == nil {
		return fmt.Errorf("app configuration is empty")
	}

	for _, r := range data.Regions {
		if !cfg.IsAllowedRegion(r) {
			return fmt.Errorf("app '%v' tried to write unauthorized region: '%v'", cfg.AppPackageName, r)
		}
	}

	// no error - application didn't try to write for regions that it isn't allowed
	return nil
}

// VerifySafetyNet verifies the Android SafetyNet device attestation against the
// allowed configuration for the application.
func VerifySafetyNet(ctx context.Context, requestTime time.Time, cfg *authorizedapp.AuthorizedApp, publish *publishmodel.Publish) error {
	if cfg == nil {
		return fmt.Errorf("cannot enforce SafetyNet, missing config")
	}

	opts := android.VerifyOptsFor(cfg, requestTime, publish.AndroidNonce())
	if err := androidValidateAttestation(ctx, publish.DeviceVerificationPayload, opts); err != nil {
		return fmt.Errorf("android.ValidateAttestation: %w", err)
	}

	return nil
}

// VerifyDeviceCheck verifies an iOS DeviceCheck token against the Apple API.
func VerifyDeviceCheck(ctx context.Context, cfg *authorizedapp.AuthorizedApp, data *publishmodel.Publish) error {
	if cfg == nil {
		return fmt.Errorf("cannot enforce DeviceCheck, missing config")
	}

	opts := &ios.VerifyOpts{
		KeyID:      cfg.DeviceCheckKeyID,
		TeamID:     cfg.DeviceCheckTeamID,
		PrivateKey: cfg.DeviceCheckPrivateKey,
	}

	if err := iosValidateDeviceToken(ctx, data.DeviceVerificationPayload, opts); err != nil {
		return fmt.Errorf("ios.ValidateDeviceToken: %w", err)
	}

	return nil
}
