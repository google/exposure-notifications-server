// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, softwar
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package authorizedapps

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
)

type formData struct {
	// Top Level
	FormKey string `form:"Key"`
	Action  string `form:"TODO"`

	// Authorized App Data
	AppPackageName              string `form:"AppPackageName"`
	Platform                    string `form:"Platform"`
	AllowedRegions              string `form:"Regions"`
	SafetyNetDisabled           bool   `form:"SafetyNetDisabled"`
	SafetyNetApkDigestSHA256    string `form:"SafetyNetApkDigestSHA256"`
	SafetyNetBasicIntegrity     bool   `form:"SafetyNetBasicIntegrity"`
	SafetyNetCTSProfileMatch    bool   `form:"SafetyNetCTSProfileMatch"`
	SafetyNetPastTime           string `form:"SafetyNetPastTime"`
	SafetyNetFutureTime         string `form:"SafetyNetFutureTime"`
	DeviceCheckDisabled         bool   `form:"DeviceCheckDisabled"`
	DeviceCheckKeyID            string `form:"DeviceCheckKeyID"`
	DeviceCheckTeamID           string `form:"DeviceCheckTeamID"`
	DeviceCheckPrivateKeySecret string `form:"DeviceCheckPrivateKeySecret"`
}

func (f *formData) PriorKey() string {
	if f.FormKey != "" {
		bytes, err := base64.StdEncoding.DecodeString(f.FormKey)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
	return ""
}

func (f *formData) PopulateAuthorizedApp(a *model.AuthorizedApp) error {
	a.AppPackageName = f.AppPackageName
	a.Platform = f.Platform
	a.AllowedRegions = make(map[string]struct{})
	for _, region := range strings.Split(f.AllowedRegions, "\n") {
		a.AllowedRegions[strings.TrimSpace(region)] = struct{}{}
	}
	// SafetyNet
	a.SafetyNetDisabled = f.SafetyNetDisabled
	a.SafetyNetApkDigestSHA256 = strings.Split(f.SafetyNetApkDigestSHA256, "\n")
	for i, s := range a.SafetyNetApkDigestSHA256 {
		a.SafetyNetApkDigestSHA256[i] = strings.TrimSpace(s)
	}
	a.SafetyNetBasicIntegrity = f.SafetyNetBasicIntegrity
	a.SafetyNetCTSProfileMatch = f.SafetyNetCTSProfileMatch
	var err error
	a.SafetyNetPastTime, err = time.ParseDuration(f.SafetyNetPastTime)
	if err != nil {
		return fmt.Errorf("failed to parse durection for SafetyNetPastTime: %w", err)
	}
	a.SafetyNetFutureTime, err = time.ParseDuration(f.SafetyNetFutureTime)
	if err != nil {
		return fmt.Errorf("failed to parse durection for SafetyNetFutureTime: %w", err)
	}
	// DeviceCheck
	a.DeviceCheckDisabled = f.DeviceCheckDisabled
	a.DeviceCheckKeyID = f.DeviceCheckKeyID
	a.DeviceCheckTeamID = f.DeviceCheckTeamID
	a.DeviceCheckPrivateKeySecret = f.DeviceCheckPrivateKeySecret
	return nil
}
