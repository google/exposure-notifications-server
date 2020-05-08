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
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/android"

	"github.com/google/go-cmp/cmp"
)

func TestBaseAPIConfig(t *testing.T) {

	cfg := New()
	if cfg.IsIOS() {
		t.Errorf("cfg.IoIOS, got true, want false")
	}
	if cfg.IsAndroid() {
		t.Errorf("cfg.IoAndroid, got true, want false")
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func TestVerifyOpts(t *testing.T) {
	testTime := time.Date(2020, 1, 13, 5, 6, 4, 6, time.Local)

	cases := []struct {
		cfg  *APIConfig
		opts android.VerifyOpts
	}{
		{
			cfg: &APIConfig{
				AppPackageName:    "foo",
				EnforceApkDigest:  false,
				CTSProfileMatch:   true,
				BasicIntegrity:    true,
				AllowedPastTime:   durationPtr(time.Duration(15 * time.Minute)),
				AllowedFutureTime: durationPtr(time.Duration(1 * time.Second)),
			},
			opts: android.VerifyOpts{
				AppPkgName:      "foo",
				CTSProfileMatch: true,
				BasicIntegrity:  true,
				MinValidTime:    timePtr(testTime.Add(-15 * time.Minute)),
				MaxValidTime:    timePtr(testTime.Add(1 * time.Second)),
			},
		},
		{
			cfg: &APIConfig{
				AppPackageName:    "foo",
				EnforceApkDigest:  false,
				CTSProfileMatch:   false,
				BasicIntegrity:    true,
				AllowedPastTime:   nil,
				AllowedFutureTime: nil,
			},
			opts: android.VerifyOpts{
				AppPkgName:      "foo",
				CTSProfileMatch: false,
				BasicIntegrity:  true,
				MinValidTime:    nil,
				MaxValidTime:    nil,
			},
		},
		{
			cfg: &APIConfig{
				AppPackageName:    "foo",
				ApkDigestSHA256:   "bar",
				EnforceApkDigest:  true,
				CTSProfileMatch:   false,
				BasicIntegrity:    true,
				AllowedPastTime:   nil,
				AllowedFutureTime: nil,
			},
			opts: android.VerifyOpts{
				AppPkgName:      "foo",
				APKDigest:       "bar",
				CTSProfileMatch: false,
				BasicIntegrity:  true,
				MinValidTime:    nil,
				MaxValidTime:    nil,
			},
		},
	}

	for i, tst := range cases {
		got := tst.cfg.VerifyOpts(testTime)
		if diff := cmp.Diff(tst.opts, got); diff != "" {
			t.Errorf("%v verify opts (-want +got):\n%v", i, diff)
		}
	}
}
