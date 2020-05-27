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

package exports

import (
	"time"

	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/export/model"
)

type formData struct {
	Region       string        `form:"Region"`
	BucketName   string        `form:"BucketName"`
	FilenameRoot string        `form:"FilenameRoot"`
	Period       time.Duration `form:"Period"`
	FromDate     string        `form:"fromdate"`
	FromTime     string        `form:"fromtime"`
	ThruDate     string        `form:"thrudate"`
	ThruTime     string        `form:"thrutime"`
	SigInfoIDs   []int64       `form:"siginfo"`
}

func (f *formData) PopulateExportConfig(ec *model.ExportConfig) error {
	from, err := admin.CombineDateAndTime(f.FromDate, f.FromTime)
	if err != nil {
		return err
	}
	thru, err := admin.CombineDateAndTime(f.ThruDate, f.ThruTime)
	if err != nil {
		return err
	}

	ec.BucketName = f.BucketName
	ec.FilenameRoot = f.FilenameRoot
	ec.Period = f.Period
	ec.Region = f.Region
	ec.From = from
	ec.Thru = thru
	ec.SignatureInfoIDs = f.SigInfoIDs

	return nil
}
