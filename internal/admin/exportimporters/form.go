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

// Package exportimporters is part of the admin system.
package exportimporters

import (
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
)

type formData struct {
	IndexFile  string `form:"index_file"`
	ExportRoot string `form:"export_root"`
	Region     string `form:"region"`

	// FromDate and FromTime are combined into FromTimestamp.
	FromDate string `form:"from_date"`
	FromTime string `form:"from_time"`

	// ThruDate and ThruTime are combined into ThruTimestamp.
	ThruDate string `form:"thru_date"`
	ThruTime string `form:"thru_time"`
}

// BuildExportImporterModel populates and mutates the given model with form
// data. It overwrites any form data that's present.
func (f *formData) BuildExportImporterModel(c *model.ExportImport) error {
	from, err := admin.CombineDateAndTime(f.FromDate, f.FromTime)
	if err != nil {
		return err
	}
	thru, err := admin.CombineDateAndTime(f.ThruDate, f.ThruTime)
	if err != nil {
		return err
	}

	if val := f.IndexFile; val != "" {
		c.IndexFile = val
	}

	if val := f.ExportRoot; val != "" {
		c.ExportRoot = val
	}

	if val := f.Region; val != "" {
		c.Region = val
	}

	if !from.IsZero() {
		c.From = from
	}

	if !thru.IsZero() {
		c.Thru = &thru
	}

	return nil
}
