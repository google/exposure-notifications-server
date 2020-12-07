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

// Package exports is part of the admin system.
package exports

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/export/model"
)

var ErrCannotSetBothTravelers = errors.New("cannot have both 'include travelers', and 'only non-travelers' set")

type formData struct {
	OutputRegion       string        `form:"OutputRegion"`
	InputRegions       string        `form:"InputRegions"`
	IncludeTravelers   bool          `form:"IncludeTravelers"`
	OnlyNonTravelers   bool          `form:"OnlyNonTravelers"`
	ExcludeRegions     string        `form:"ExcludeRegions"`
	BucketName         string        `form:"BucketName"`
	FilenameRoot       string        `form:"FilenameRoot"`
	Period             time.Duration `form:"Period"`
	FromDate           string        `form:"fromdate"`
	FromTime           string        `form:"fromtime"`
	ThruDate           string        `form:"thrudate"`
	ThruTime           string        `form:"thrutime"`
	SigInfoIDs         []int64       `form:"siginfo"`
	MaxRecordsOverride int           `form:"MaxRecordsOverride"`
}

// splitRegions turns a string of regions (generally separated by newlines), and
// breaks them up into an alphabetically sorted slice of strings.
func splitRegions(regions string) []string {
	ret := make([]string, 0, 20)
	for _, s := range strings.Split(regions, "\n") {
		s := strings.TrimSpace(s)
		if s != "" {
			ret = append(ret, s)
		}
	}
	sort.Strings(ret)
	return ret
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
	if f.IncludeTravelers && f.OnlyNonTravelers {
		return ErrCannotSetBothTravelers
	}

	ec.BucketName = strings.TrimSpace(f.BucketName)
	ec.FilenameRoot = strings.TrimSpace(f.FilenameRoot)
	ec.Period = f.Period
	ec.OutputRegion = strings.TrimSpace(f.OutputRegion)
	ec.InputRegions = splitRegions(f.InputRegions)
	ec.IncludeTravelers = f.IncludeTravelers
	ec.OnlyNonTravelers = f.OnlyNonTravelers
	ec.ExcludeRegions = splitRegions(f.ExcludeRegions)
	ec.From = from
	ec.Thru = thru
	ec.SignatureInfoIDs = f.SigInfoIDs
	if f.MaxRecordsOverride > 0 {
		ec.MaxRecordsOverride = &f.MaxRecordsOverride
	} else {
		ec.MaxRecordsOverride = nil
	}

	if len(ec.SignatureInfoIDs) > 10 {
		return fmt.Errorf("too many signing keys selected, there is a limit of 10")
	}

	return nil
}
