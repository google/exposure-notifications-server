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
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/android"
	"github.com/google/exposure-notifications-server/internal/model"
)

const (
	appPkgName = "com.example.pkg"
)

func TestVerifyRegions(t *testing.T) {
	allRegions := &model.APIConfig{
		AppPackageName:  appPkgName,
		AllowAllRegions: true,
	}
	usCaRegions := &model.APIConfig{
		AppPackageName: appPkgName,
		AllowedRegions: make(map[string]struct{}),
	}
	usCaRegions.AllowedRegions["US"] = struct{}{}
	usCaRegions.AllowedRegions["CA"] = struct{}{}

	cases := []struct {
		Data *model.Publish
		Msg  string
		Cfg  *model.APIConfig
	}{
		{
			&model.Publish{Regions: []string{"US"}},
			"no allowed regions configured",
			nil,
		},
		{
			&model.Publish{Regions: []string{"US"}},
			"",
			allRegions,
		},
		{
			&model.Publish{Regions: []string{"US"}},
			"",
			usCaRegions,
		},
		{
			&model.Publish{Regions: []string{"US", "CA"}},
			"",
			usCaRegions,
		},
		{
			&model.Publish{Regions: []string{"MX"}},
			fmt.Sprintf("application '%v' tried to write unauthorized region: '%v'", appPkgName, "MX"),
			usCaRegions,
		},
	}

	for i, c := range cases {
		err := VerifyRegions(c.Cfg, c.Data)
		if c.Msg == "" && err == nil {
			continue
		}
		if c.Msg == "" && err != nil {
			t.Errorf("%v got %v, wanted no error", i, err)
			continue
		}
		if err.Error() != c.Msg {
			t.Errorf("%v wrong error, got %v, want %v", i, err, c.Msg)
		}
	}
}

func TestVerifySafetyNet(t *testing.T) {
	allRegions := &model.APIConfig{
		AppPackageName:  appPkgName,
		AllowAllRegions: true,
	}
	allRegionsSafetyCheckDisabled := &model.APIConfig{
		AppPackageName:  appPkgName,
		AllowAllRegions: true,
		BypassSafetyNet: true,
	}

	cases := []struct {
		Data              *model.Publish
		Msg               string
		Cfg               *model.APIConfig
		AttestationResult error
	}{
		{
			// With no configuration, return err.
			&model.Publish{Regions: []string{"US"}},
			"cannot enforce safetynet, no application config",
			nil,
			nil,
		}, {
			// Verify when Validate Attestation Passes, return nil.
			&model.Publish{Regions: []string{"US"}},
			"",
			allRegions,
			nil,
		}, {
			// Verify when ValidateAttestation raises err, with safety check
			// enabled, return err.
			&model.Publish{Regions: []string{"US"}},
			"android.ValidateAttestation: mocked",
			allRegions,
			fmt.Errorf("mocked"),
		}, {
			// Verify when ValidateAttestation raises err, with safety check
			// disabled, return nil.
			&model.Publish{Regions: []string{"US"}},
			"",
			allRegionsSafetyCheckDisabled,
			fmt.Errorf("mocked"),
		},
	}

	for i, c := range cases {
		var ctx = context.Background()
		androidValidateAttestation = func(context.Context, string, android.VerifyOpts) error {
			return c.AttestationResult
		}

		err := VerifySafetyNet(ctx, time.Now(), c.Cfg, c.Data)
		if c.Msg == "" && err == nil {
			continue
		}
		if c.Msg == "" && err != nil {
			t.Errorf("%v got %v, wanted no error", i, err)
			continue
		}
		if err.Error() != c.Msg {
			t.Errorf("%v wrong error, got %v, want %v", i, err, c.Msg)
		}
	}
}
