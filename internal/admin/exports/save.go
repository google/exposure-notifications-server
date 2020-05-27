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
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

type saveController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewSave(c *admin.Config, env *serverenv.ServerEnv) *saveController {
	return &saveController{config: c, env: env}
}

func (v *saveController) Execute(c *gin.Context) {
	var form formData
	err := c.Bind(&form)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	exportDB := database.New(v.env.Database())
	exportConfig := &model.ExportConfig{}
	idParam := c.Param("id")
	if idParam != "0" {
		cfgID, err := strconv.ParseInt(idParam, 10, 64)
		if err != nil {
			admin.ErrorPage(c, "unable to parse `id` param.")
			return
		}
		exportConfig, err = exportDB.GetExportConfig(ctx, cfgID)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error loading export config: %v", err))
			return
		}
	}

	if err := form.PopulateExportConfig(exportConfig); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("error processing export config: %v", err))
	}

	updateFn := exportDB.AddExportConfig
	if exportConfig.ConfigID != 0 {
		updateFn = exportDB.UpdateExportConfig
	}
	if err := updateFn(ctx, exportConfig); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("Error writing export config: %v", err))
		return
	}

	m.AddSuccess(fmt.Sprintf("Updated export config #%v", exportConfig.ConfigID))

	usedSigInfos := make(map[int64]bool)
	for _, id := range exportConfig.SignatureInfoIDs {
		usedSigInfos[id] = true
	}

	sigInfos, err := exportDB.ListAllSigntureInfos(ctx)
	if err != nil {
		admin.ErrorPage(c, fmt.Sprintf("Error reading the database: %v", err))
	}

	m["export"] = exportConfig
	m["usedSigInfos"] = usedSigInfos
	m["siginfos"] = sigInfos
	c.HTML(http.StatusOK, "export", m)
}
