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

	"github.com/google/exposure-notifications-server/internal/android"
)

const (
	iosDevice     = "ios"
	androidDevice = "android"
)

// APIConfig represents the configuration for a single exposure notification
// application and their access to and requirements for using the API.
// DB times of 0 are interpreted to be "unbounded" in that direction.
type APIConfig struct {
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

func NewAPIConfig() (*APIConfig, error) {
	return &APIConfig{
		AllowedRegions: make(map[string]struct{}),
	}, nil
}

// IsIOS returns true if the platform is equal to `iosDevice`
func (c *APIConfig) IsIOS() bool {
	return c.Platform == iosDevice
}

// IsAndroid returns true if the platform is equal to `android`
func (c *APIConfig) IsAndroid() bool {
	return c.Platform == androidDevice
}

// IsAllowedRegion returns true if the region is in the list of allowed regions,
// false otherwise.
func (c *APIConfig) IsAllowedRegion(s string) bool {
	_, ok := c.AllowedRegions[s]
	return ok
}

// VerifyOpts returns the Android SafetyNet verification options to be used
// based on the API config.
func (c *APIConfig) VerifyOpts(from time.Time, nonce string) android.VerifyOpts {
	digests := make([]string, len(c.ApkDigestSHA256))
	copy(digests, c.ApkDigestSHA256)
	rtn := android.VerifyOpts{
		AppPkgName:      c.AppPackageName,
		CTSProfileMatch: c.CTSProfileMatch,
		BasicIntegrity:  c.BasicIntegrity,
		APKDigest:       digests,
		Nonce:           nonce,
	}

	// Calculate the valid time window based on now + config options.
	if c.AllowedPastTime > 0 {
		minTime := from.Add(-c.AllowedPastTime)
		rtn.MinValidTime = minTime
	}
	if c.AllowedFutureTime > 0 {
		maxTime := from.Add(c.AllowedFutureTime)
		rtn.MaxValidTime = maxTime
	}

	return rtn
}
