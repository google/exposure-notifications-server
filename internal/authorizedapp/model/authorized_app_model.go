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

package model

import (
	"crypto/ecdsa"
	"strings"
	"time"
)

const (
	BothPlatforms = "both"
	IosDevice     = "ios"
	AndroidDevice = "android"
)

// AuthorizedApp represents the configuration for a single exposure notification
// application and their access to and requirements for using the API. DB times
// of 0 are interpreted to be "unbounded" in that direction.
type AuthorizedApp struct {
	// AppPackageName is the name of the package like com.company.app.
	AppPackageName string

	// Platform is the app platform like "android" or "ios".
	// Deprecated - Platform doesn't matter with device attestations going away.
	Platform string

	// AllowedRegions is the list of allowed regions for this app. If the list is
	// empty, all regions are permitted.
	AllowedRegions map[string]struct{}

	// AllowedHealthAuthorityIDs represents the set of allowed health authorities
	// that this app can obtain and verify diagnosis verification certificates from.
	AllowedHealthAuthorityIDs map[int64]struct{}

	// DEPRECATION NOTICE:
	// Everything below here is deprecated.
	// SafetyNet configuration.
	SafetyNetDisabled        bool
	SafetyNetApkDigestSHA256 []string
	SafetyNetBasicIntegrity  bool
	SafetyNetCTSProfileMatch bool
	SafetyNetPastTime        time.Duration
	SafetyNetFutureTime      time.Duration

	// DeviceCheck configuration.
	DeviceCheckDisabled         bool
	DeviceCheckKeyID            string
	DeviceCheckTeamID           string
	DeviceCheckPrivateKey       *ecdsa.PrivateKey
	DeviceCheckPrivateKeySecret string
}

func NewAuthorizedApp() *AuthorizedApp {
	return &AuthorizedApp{
		AllowedRegions:            make(map[string]struct{}),
		AllowedHealthAuthorityIDs: make(map[int64]struct{}),
	}
}

func (c *AuthorizedApp) AllAllowedRegions() []string {
	regions := []string{}
	for k := range c.AllowedRegions {
		regions = append(regions, k)
	}
	return regions
}

func (c *AuthorizedApp) AllAllowedHealthAuthorityIDs() []int64 {
	has := []int64{}
	for k := range c.AllowedHealthAuthorityIDs {
		has = append(has, k)
	}
	return has
}

func (c *AuthorizedApp) Validate() []string {
	errors := make([]string, 0)
	if c.AppPackageName == "" {
		errors = append(errors, "AppPackageName cannot be empty")
	}
	if !(c.Platform == "android" || c.Platform == "ios" || c.Platform == "both") {
		errors = append(errors, "platform must be one if {'android', 'ios', or 'both'}")
	}
	if !c.SafetyNetDisabled {
		if c.SafetyNetPastTime < 0 {
			errors = append(errors, "SafetyNetPastTime cannot be negative")
		}
		if c.SafetyNetFutureTime < 0 {
			errors = append(errors, "SafetyNetFutureTime cannot be negative")
		}
	}
	if !c.DeviceCheckDisabled {
		if c.DeviceCheckKeyID == "" {
			errors = append(errors, "When DeviceCheck is enabled, DeviceCheckKeyID cannot be empty")
		}
		if c.DeviceCheckTeamID == "" {
			errors = append(errors, "When DeviceCheck is enabled, DeviceCheckTeamID cannot be empty")
		}
		if c.DeviceCheckPrivateKeySecret == "" {
			errors = append(errors, "When DeviceCheck is enabled, DeviceCheckPrivateKeySecret cannot be empty")
		}
	}
	return errors
}

// IsIOS returns true if the platform is equal to `iosDevice`
func (c *AuthorizedApp) IsIOS() bool {
	return c.Platform == IosDevice
}

// IsAndroid returns true if the platform is equal to `android`
func (c *AuthorizedApp) IsAndroid() bool {
	return c.Platform == AndroidDevice
}

// IsDualPlatform returns true if the platform is equal to 'both'
// i.e. AppPackageName and BundleID are the same.
func (c *AuthorizedApp) IsDualPlatform() bool {
	return c.Platform == BothPlatforms
}

func (c *AuthorizedApp) RegionsOnePerLine() string {
	regions := []string{}
	for r := range c.AllowedRegions {
		regions = append(regions, r)
	}
	return strings.Join(regions, "\n")
}

func (c *AuthorizedApp) APKDigestOnePerLine() string {
	return strings.Join(c.SafetyNetApkDigestSHA256, "\n")
}

// IsAllowedRegion returns true if the regions list is empty or if the given
// region is in the list of allowed regions.
func (c *AuthorizedApp) IsAllowedRegion(s string) bool {
	if len(c.AllowedRegions) == 0 {
		return true
	}

	_, ok := c.AllowedRegions[s]
	return ok
}
