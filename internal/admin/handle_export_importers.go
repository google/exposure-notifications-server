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
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

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
		record, err := s.getExportImporter(ctx, db, c.Param("id"))
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Failed to load export importer: %s", err))
			return
		}

		if err := form.BuildExportImporterModel(record); err != nil {
			ErrorPage(c, fmt.Sprintf("failed to build export importer config: %s", err))
			return
		}

		fn := db.AddConfig
		if record.ID != 0 {
			fn = db.UpdateConfig
		}

		if err := fn(ctx, record); err != nil {
			ErrorPage(c, fmt.Sprintf("failed to write export importer config: %s", err))
			return
		}

		m.AddSuccess("Successfully updated export importer config!")

		c.Redirect(http.StatusSeeOther, fmt.Sprintf("/export-importers/%d", record.ID))
		c.Abort()
	}
}

// HandleExportImportersShow handles the create/update actions for export
// importers.
func (s *Server) HandleExportImportersShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		db := database.New(s.env.Database())
		record, err := s.getExportImporter(ctx, db, c.Param("id"))
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Failed to load export importer: %s", err))
			return
		}

		// Load public keys
		var publicKeys []*model.ImportFilePublicKey
		if c.Param("id") != "0" {
			publicKeys, err = db.AllPublicKeys(ctx, record)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Failed to load public keys: %s", err))
				return
			}
		}

		m := make(TemplateMap)
		m.AddTitle(fmt.Sprintf("import %q", record.IndexFile))
		m["model"] = record
		m["keys"] = publicKeys
		m["newkey"] = &model.ImportFilePublicKey{}
		c.HTML(http.StatusOK, "export-importer", m)
		c.Abort()
	}
}

// getExportImporter gets an export importer with the given id. If the id is ""
// or "0", an empty record is returned. Otherwise, it attempts to find a record
// with the id.
func (s *Server) getExportImporter(ctx context.Context, db *database.ExportImportDB, idRaw string) (*model.ExportImport, error) {
	if idRaw == "0" {
		return &model.ExportImport{}, nil
	}

	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q as int: %w", idRaw, err)
	}

	return db.GetConfig(ctx, id)
}

type exportImporterFormData struct {
	IndexFile  string `form:"index-file"`
	ExportRoot string `form:"export-root"`
	Region     string `form:"region"`
	Travelers  bool   `form:"travelers"`

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
		return fmt.Errorf("invalid from time: %w", err)
	}
	thru, err := CombineDateAndTime(f.ThruDate, f.ThruTime)
	if err != nil {
		return fmt.Errorf("invalid thru time: %w", err)
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

	c.Traveler = f.Travelers

	if !from.IsZero() {
		c.From = from
	} else {
		c.From = time.Now().UTC().Add(-1 * time.Minute)
	}

	if !thru.IsZero() {
		c.Thru = &thru
	}

	return nil
}
