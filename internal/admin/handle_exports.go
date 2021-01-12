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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
)

// HandleExportsSave handles the create/update actions for exports.
func (s *Server) HandleExportsSave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form exportFormData
		err := c.Bind(&form)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		db := database.New(s.env.Database())
		record, err := s.getExportConfig(ctx, db, c.Param("id"))
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Failed to load export config: %s", err))
			return
		}

		if err := form.PopulateExportConfig(record); err != nil {
			ErrorPage(c, fmt.Sprintf("error processing export config: %v", err))
		}

		updateFn := db.AddExportConfig
		if record.ConfigID != 0 {
			updateFn = db.UpdateExportConfig
		}
		if err := updateFn(ctx, record); err != nil {
			ErrorPage(c, fmt.Sprintf("Error writing export config: %v", err))
			return
		}

		m.AddSuccess(fmt.Sprintf("Updated export config #%v", record.ConfigID))

		usedSigInfos := make(map[int64]bool)
		for _, id := range record.SignatureInfoIDs {
			usedSigInfos[id] = true
		}

		sigInfos, err := db.ListAllSignatureInfos(ctx)
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Error reading the database: %v", err))
		}

		m["export"] = record
		m["usedSigInfos"] = usedSigInfos
		m["siginfos"] = sigInfos
		c.HTML(http.StatusOK, "export", m)
	}
}

// HandleExportsShow handles the show action for exports.
func (s *Server) HandleExportsShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		db := database.New(s.env.Database())
		record, err := s.getExportConfig(ctx, db, c.Param("id"))
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Failed to load export config: %s", err))
			return
		}

		usedSigInfos := make(map[int64]bool)
		for _, id := range record.SignatureInfoIDs {
			usedSigInfos[id] = true
		}

		sigInfos, err := db.ListAllSignatureInfos(ctx)
		if err != nil {
			ErrorPage(c, fmt.Sprintf("Error reading the database: %v", err))
		}

		m["export"] = record
		m["usedSigInfos"] = usedSigInfos
		m["siginfos"] = sigInfos
		c.HTML(http.StatusOK, "export", m)
	}
}

// getExportConfig gets an export config with the given id. If the id is "" or
// "0", an empty record is returned. Otherwise, it attempts to find a record
// with the id.
func (s *Server) getExportConfig(ctx context.Context, db *database.ExportDB, idRaw string) (*model.ExportConfig, error) {
	if idRaw == "0" {
		return &model.ExportConfig{
			Period:           24 * time.Hour,
			IncludeTravelers: true,
		}, nil
	}

	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q as int: %w", idRaw, err)
	}

	return db.GetExportConfig(ctx, id)
}

type exportFormData struct {
	OutputRegion       string        `form:"output-region"`
	InputRegions       string        `form:"input-regions"`
	IncludeTravelers   bool          `form:"include-travelers"`
	OnlyNonTravelers   bool          `form:"only-non-travelers"`
	ExcludeRegions     string        `form:"exclude-regions"`
	BucketName         string        `form:"bucket-name"`
	FilenameRoot       string        `form:"filename-root"`
	Period             time.Duration `form:"period"`
	FromDate           string        `form:"from-date"`
	FromTime           string        `form:"from-time"`
	ThruDate           string        `form:"thru-date"`
	ThruTime           string        `form:"thru-time"`
	SigInfoIDs         []int64       `form:"sig-info"`
	MaxRecordsOverride int           `form:"max-records-override"`
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

func (f *exportFormData) PopulateExportConfig(ec *model.ExportConfig) error {
	from, err := CombineDateAndTime(f.FromDate, f.FromTime)
	if err != nil {
		return err
	}
	thru, err := CombineDateAndTime(f.ThruDate, f.ThruTime)
	if err != nil {
		return err
	}
	if f.IncludeTravelers && f.OnlyNonTravelers {
		return fmt.Errorf("cannot have both 'include travelers', and 'only non-travelers' set")
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
