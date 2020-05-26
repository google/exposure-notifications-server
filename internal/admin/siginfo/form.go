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

package siginfo

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/export/model"
)

type formData struct {
	SigningKey        string `form:"SigningKey"`
	EndDate           string `form:"enddate"`
	EndTime           string `form:"endtime"`
	SigningKeyID      string `form:"SigningKeyID"`
	SigningKeyVersion string `form:"SigningKeyVersion"`
	AppPackageName    string `form:"AppPackageName"`
	BundleID          string `form:"BundleID"`
}

func (f *formData) EndTimestamp() (time.Time, error) {
	if f.EndDate == "" {
		// Return zero time.
		return time.Time{}, nil
	}
	if f.EndTime == "" {
		f.EndTime = "00:00"
	}
	return time.Parse("2006-01-02 15:04", f.EndDate+" "+f.EndTime)
}

func (f *formData) PopulateSigInfo(si *model.SignatureInfo) error {
	ts, err := f.EndTimestamp()
	if err != nil {
		return err
	}

	si.SigningKey = f.SigningKey
	si.AppPackageName = f.AppPackageName
	si.BundleID = f.BundleID
	si.SigningKeyVersion = f.SigningKeyVersion
	si.SigningKeyID = f.SigningKeyID
	si.EndTimestamp = ts
	return nil
}
