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

package database

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
	AppPackageName  string
	Platform        string
	AllowedRegions  map[string]struct{}
	AllowAllRegions bool

	// SafetyNet configuration.
	// TODO(sethvargo): Rename these to clarify they are for SafetyNet.
	AllowedPastTime   time.Duration
	AllowedFutureTime time.Duration
	ApkDigestSHA256   []string
	BasicIntegrity    bool
	CTSProfileMatch   bool

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

// IsAllowedRegion returns true if the region is in the list of allowed regions,
// false otherwise.
func (c *AuthorizedApp) IsAllowedRegion(s string) bool {
	_, ok := c.AllowedRegions[s]
	return ok
}
