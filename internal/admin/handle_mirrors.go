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
	"github.com/google/exposure-notifications-server/internal/mirror/database"
	"github.com/google/exposure-notifications-server/internal/mirror/model"
)

// HandleMirrorsSave handles the create/update actions for mirrors.
func (s *Server) HandleMirrorsSave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form mirrorFormData
		if err := c.Bind(&form); err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		db := database.New(s.env.Database())
		mirror := &model.Mirror{}
		if idParam := c.Param("id"); idParam != "0" {
			cfgID, err := strconv.ParseInt(idParam, 10, 64)
			if err != nil {
				ErrorPage(c, "unable to parse `id` param.")
				return
			}
			mirror, err = db.GetMirror(ctx, cfgID)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error loading mirror: %v", err))
				return
			}
		}

		switch form.Action {
		case "delete":
			if err := db.DeleteMirror(ctx, mirror); err != nil {
				ErrorPage(c, fmt.Sprintf("Failed to delete mirror: %v", err))
				return
			}

			m.AddSuccess(fmt.Sprintf("Deleted mirror %d", mirror.ID))
			c.Redirect(http.StatusSeeOther, "/")
		case "save":
			if err := form.PopulateMirror(mirror); err != nil {
				ErrorPage(c, fmt.Sprintf("error processing mirror: %v", err))
				return
			}

			updateFn := db.AddMirror
			if mirror.ID != 0 {
				updateFn = db.UpdateMirror
			}
			if err := updateFn(ctx, mirror); err != nil {
				ErrorPage(c, fmt.Sprintf("Error writing mirror: %v", err))
				return
			}

			m.AddSuccess(fmt.Sprintf("Updated mirror %d", mirror.ID))

			m["mirror"] = mirror
			c.HTML(http.StatusOK, "mirror", m)
		default:
			ErrorPage(c, "Invalid form action")
		}
	}
}

// HandleMirrorsShow handles the show action for mirrors.
func (s *Server) HandleMirrorsShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		db := database.New(s.env.Database())
		mirror := &model.Mirror{}
		if idParam := c.Param("id"); idParam != "0" {
			id, err := strconv.ParseInt(idParam, 10, 64)
			if err != nil {
				ErrorPage(c, "unable to parse `id` param.")
				return
			}
			mirror, err = db.GetMirror(ctx, id)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error loading mirror: %v", err))
				return
			}
		}

		var mirrorFiles []*model.MirrorFile
		if mirror.ID != 0 {
			var err error
			mirrorFiles, err = db.ListFiles(ctx, mirror.ID)
			if err != nil {
				ErrorPage(c, fmt.Sprintf("Error loading mirror files: %v", err))
				return
			}
		}

		m["mirror"] = mirror
		m["mirrorFiles"] = mirrorFiles
		c.HTML(http.StatusOK, "mirror", m)
	}
}

type mirrorFormData struct {
	Action string `form:"action" binding:"required"`

	IndexFile          string `form:"index-file" binding:"required"`
	ExportRoot         string `form:"export-root"`
	CloudStorageBucket string `form:"cloud-storage-bucket" binding:"required"`
	FilenameRoot       string `form:"filename-root"`
	FilenameRewrite    string `form:"filename-rewrite"`
}

func (f *mirrorFormData) PopulateMirror(m *model.Mirror) error {
	m.IndexFile = f.IndexFile
	m.ExportRoot = f.ExportRoot
	m.CloudStorageBucket = f.CloudStorageBucket
	m.FilenameRoot = f.FilenameRoot
	if f.FilenameRewrite != "" {
		m.FilenameRewrite = &f.FilenameRewrite
	} else {
		m.FilenameRewrite = nil
	}
	return nil
}
