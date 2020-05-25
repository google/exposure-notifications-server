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
	"strings"
	"testing"
	"time"

	"github.com/google/exposure-notifications-server/internal/android"
	authorizedapp "github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/publish/model"
)

const (
	appPkgName = "com.example.pkg"
)

func TestVerifyRegions(t *testing.T) {
	cases := []struct {
		name string
		data *model.Publish
		cfg  *authorizedapp.AuthorizedApp
		err  bool
	}{
		{
			name: "nil_config",
			data: &model.Publish{Regions: []string{"US"}},
			cfg:  nil,
			err:  true,
		},
		{
			name: "nil_regions_allows_all",
			data: &model.Publish{Regions: []string{"US"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
			},
		},
		{
			name: "empty_regions_allows_all",
			data: &model.Publish{Regions: []string{"US"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
				AllowedRegions: map[string]struct{}{},
			},
		},
		{
			name: "region_matches_one",
			data: &model.Publish{Regions: []string{"US"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
				AllowedRegions: map[string]struct{}{
					"US": {},
					"CA": {},
				},
			},
		},
		{
			name: "region_matches_all",
			data: &model.Publish{Regions: []string{"US", "CA"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
				AllowedRegions: map[string]struct{}{
					"US": {},
					"CA": {},
				},
			},
		},
		{
			name: "region_matches_some",
			data: &model.Publish{Regions: []string{"US", "MX"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
				AllowedRegions: map[string]struct{}{
					"US": {},
					"CA": {},
				},
			},
			err: true,
		},
		{
			name: "region_matches_none",
			data: &model.Publish{Regions: []string{"MX"}},
			cfg: &authorizedapp.AuthorizedApp{
				AppPackageName: appPkgName,
				AllowedRegions: map[string]struct{}{
					"US": {},
					"CA": {},
				},
			},
			err: true,
		},
	}

	for _, tc := range cases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyRegions(tc.cfg, tc.data); (err != nil) != tc.err {
				t.Fatal(err)
			}
		})
	}
}

func TestVerifySafetyNet(t *testing.T) {
	allRegions := &authorizedapp.AuthorizedApp{
		AppPackageName: appPkgName,
	}

	cases := []struct {
		Data              *model.Publish
		Msg               string
		Cfg               *authorizedapp.AuthorizedApp
		AttestationResult error
	}{
		{
			// With no configuration, return err.
			&model.Publish{Regions: []string{"US"}},
			"cannot enforce SafetyNet",
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
		},
	}

	for i, c := range cases {
		var ctx = context.Background()
		androidValidateAttestation = func(context.Context, string, *android.VerifyOpts) error {
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
		if !strings.Contains(err.Error(), c.Msg) {
			t.Errorf("%v wrong error, got %v, want %v", i, err, c.Msg)
		}
	}
}
