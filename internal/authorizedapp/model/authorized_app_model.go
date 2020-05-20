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
	"time"
)

const (
	iosDevice     = "ios"
	androidDevice = "android"
)

// AuthorizedApp represents the configuration for a single exposure notification
// application and their access to and requirements for using the API. DB times
// of 0 are interpreted to be "unbounded" in that direction.
type AuthorizedApp struct {
	// AppPackageName is the name of the package like com.company.app.
	AppPackageName string

	// Platform is the app platform like "android" or "ios".
	Platform string

	// AllowedRegions is the list of allowed regions for this app. If the list is
	// empty, all regions are permitted.
	AllowedRegions map[string]struct{}

	// SafetyNet configuration.
	SafetyNetApkDigestSHA256 []string
	SafetyNetBasicIntegrity  bool
	SafetyNetCTSProfileMatch bool
	SafetyNetPastTime        time.Duration
	SafetyNetFutureTime      time.Duration

	// DeviceCheck configuration.
	DeviceCheckKeyID      string
	DeviceCheckTeamID     string
	DeviceCheckPrivateKey *ecdsa.PrivateKey
}

func NewAuthorizedApp() *AuthorizedApp {
	return &AuthorizedApp{
		AllowedRegions: make(map[string]struct{}),
	}
}

// IsIOS returns true if the platform is equal to `iosDevice`
func (c *AuthorizedApp) IsIOS() bool {
	return c.Platform == iosDevice
}

// IsAndroid returns true if the platform is equal to `android`
func (c *AuthorizedApp) IsAndroid() bool {
	return c.Platform == androidDevice
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
