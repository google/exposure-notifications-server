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
	"strings"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
)

type formData struct {
	// Top Level
	FormKey string `form:"Key"`
	Action  string `form:"TODO"`

	// Authorized App Data
	AppPackageName string `form:"AppPackageName"`
	AllowedRegions string `form:"Regions"`
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
	a.AllowedRegions = make(map[string]struct{})
	for _, region := range strings.Split(f.AllowedRegions, "\n") {
		a.AllowedRegions[strings.TrimSpace(region)] = struct{}{}
	}
	return nil
}
