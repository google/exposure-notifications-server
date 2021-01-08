// Copyright 2021 Google LLC
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

package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/exportimport/model"
)

// HandleExportImportersSave handles the create/update actions for export
// importers.
func (s *Server) HandleExportImportersSave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form exportImporterFormData
		if err := c.Bind(&form); err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		db := database.New(s.env.Database())
		model := new(model.ExportImport)

		idRaw := c.Param("id")
		if idRaw != "" && idRaw != "0" {
			id, err := strconv.ParseInt(idRaw, 10, 64)
			if err != nil {
				ErrorPage(c, "failed to to parse `id` param.")
				return
			}

			model, err = db.GetConfig(ctx, id)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("failed to load export importer config: %s", err))
				return
			}
		}

		if err := form.BuildExportImporterModel(model); err != nil {
			ErrorPage(c, fmt.Sprintf("failed to build export importer config: %s", err))
			return
		}

		fn := db.AddConfig
		if model.ID != 0 {
			fn = db.UpdateConfig
		}

		if err := fn(ctx, model); err != nil {
			ErrorPage(c, fmt.Sprintf("failed to write export importer config: %s", err))
			return
		}

		m.AddSuccess("Successfully updated export importer config!")
		m["model"] = model
		c.HTML(http.StatusOK, "export-importer", m)
		c.Abort()
	}
}

// HandleExportImportersShow handles the create/update actions for export
// importers.
func (s *Server) HandleExportImportersShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		db := database.New(s.env.Database())
		model := new(model.ExportImport)

		if idRaw := c.Param("id"); idRaw != "" && idRaw != "0" {
			id, err := strconv.ParseInt(idRaw, 10, 64)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Failed to parse `id` param: %s", err))
				return
			}

			model, err = db.GetConfig(ctx, id)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Failed to load export importer config: %s", err))
				return
			}
		}

		m := make(TemplateMap)
		m["model"] = model
		c.HTML(http.StatusOK, "export-importer", m)
		c.Abort()
	}
}

type exportImporterFormData struct {
	IndexFile  string `form:"index-file"`
	ExportRoot string `form:"export-root"`
	Region     string `form:"region"`

	// FromDate and FromTime are combined into FromTimestamp.
	FromDate string `form:"from-date"`
	FromTime string `form:"from-time"`

	// ThruDate and ThruTime are combined into ThruTimestamp.
	ThruDate string `form:"thru-date"`
	ThruTime string `form:"thru-time"`
}

// BuildExportImporterModel populates and mutates the given model with form
// data. It overwrites any form data that's present.
func (f *exportImporterFormData) BuildExportImporterModel(c *model.ExportImport) error {
	from, err := CombineDateAndTime(f.FromDate, f.FromTime)
	if err != nil {
		return err
	}
	thru, err := CombineDateAndTime(f.ThruDate, f.ThruTime)
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
