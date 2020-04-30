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
	"time"

	"github.com/googlepartners/exposure-notifications/internal/android"
)

type APIConfig struct {
	AppPackageName   string          `db:"app_package_name"`
	ApkDigestSHA256  string          `db:"apk_digest"`
	EnforceApkDigest bool            `db:"enforce_apk_digest"`
	CTSProfileMatch  bool            `db:"cts_profile_match"`
	BasicIntegrity   bool            `db:"basic_integrity"`
	MaxAgeSeconds    time.Duration   `db:"max_age_seconds"`
	ClockSkewSeconds time.Duration   `db:"clock_skew_seconds"`
	AllowedRegions   map[string]bool `db:"allowed_regions"`
	AllowAllRegions  bool            `db:"all_regions"`
	BypassSafetynet  bool            `db:"bypass_safetynet"`
}

func NewAPIConfig() *APIConfig {
	return &APIConfig{AllowedRegions: make(map[string]bool)}
}

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
		minTime := from.UTC().Add(-c.MaxAgeSeconds * time.Second)
		rtn.MinValidTime = &minTime
	}
	if c.ClockSkewSeconds > 0 {
		maxTime := from.UTC().Add(c.ClockSkewSeconds * time.Second)
		rtn.MaxValidTime = &maxTime
	}

	return rtn
}
