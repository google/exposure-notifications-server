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

// Package siginfo is part of the admin system.
package siginfo

import (
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/export/model"
)

type formData struct {
	SigningKey        string `form:"SigningKey"`
	EndDate           string `form:"enddate"`
	EndTime           string `form:"endtime"`
	SigningKeyID      string `form:"SigningKeyID"`
	SigningKeyVersion string `form:"SigningKeyVersion"`
}

func (f *formData) EndTimestamp() (time.Time, error) {
	return admin.CombineDateAndTime(f.EndDate, f.EndTime)
}

func (f *formData) PopulateSigInfo(si *model.SignatureInfo) error {
	ts, err := f.EndTimestamp()
	if err != nil {
		return err
	}

	si.SigningKey = strings.TrimSpace(f.SigningKey)
	si.SigningKeyVersion = strings.TrimSpace(f.SigningKeyVersion)
	si.SigningKeyID = f.SigningKeyID
	si.EndTimestamp = ts
	return nil
}
