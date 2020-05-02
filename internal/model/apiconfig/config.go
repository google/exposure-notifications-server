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

package apiconfig

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/android"
)

const (
	iosDevice     = "ios"
	androidDevice = "android"
)

// APIConfig represents the configuration for a single exposure notification
// application and their access to and requirements for using the API.
type APIConfig struct {
	AppPackageName   string          `db:"app_package_name"`
	Platform         string          `db:"platform"`
	ApkDigestSHA256  string          `db:"apk_digest"`
	EnforceApkDigest bool            `db:"enforce_apk_digest"`
	CTSProfileMatch  bool            `db:"cts_profile_match"`
	BasicIntegrity   bool            `db:"basic_integrity"`
	MaxAgeSeconds    int64           `db:"max_age_seconds"`
	ClockSkewSeconds int64           `db:"clock_skew_seconds"`
	AllowedRegions   map[string]bool `db:"allowed_regions"`
	AllowAllRegions  bool            `db:"all_regions"`
	BypassSafetynet  bool            `db:"bypass_safetynet"`
}

// New creates a new, empty API config
func New() *APIConfig {
	return &APIConfig{AllowedRegions: make(map[string]bool)}
}

// IsIOS returns true if the platform is equal to `iosDevice`
func (c *APIConfig) IsIOS() bool {
	return c.Platform == iosDevice
}

// IsAndroid returns true if the platform is equal to `android`
func (c *APIConfig) IsAndroid() bool {
	return c.Platform == androidDevice
}

// VerifyOpts returns the Android SafetyNet verification options to be used
// based on the API config.
func (c *APIConfig) VerifyOpts(from time.Time) android.VerifyOpts {
	rtn := android.VerifyOpts{
		AppPkgName:      c.AppPackageName,
		CTSProfileMatch: c.CTSProfileMatch,
		BasicIntegrity:  c.BasicIntegrity,
	}
	if c.EnforceApkDigest && len(c.ApkDigestSHA256) > 0 {
		rtn.APKDigest = c.ApkDigestSHA256
	}

	// Calculate the valid time window based on now + config options.
	if c.MaxAgeSeconds > 0 {
		minTime := from.UTC().Add(time.Duration(-c.MaxAgeSeconds) * time.Second)
		rtn.MinValidTime = &minTime
	}
	if c.ClockSkewSeconds > 0 {
		maxTime := from.UTC().Add(time.Duration(c.ClockSkewSeconds) * time.Second)
		rtn.MaxValidTime = &maxTime
	}

	return rtn
}
