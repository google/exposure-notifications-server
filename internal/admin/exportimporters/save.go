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

package exportimporters

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	exportimportdatabase "github.com/google/exposure-notifications-server/internal/exportimport/database"
	exportimportmodel "github.com/google/exposure-notifications-server/internal/exportimport/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

type saveController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewSave(c *admin.Config, env *serverenv.ServerEnv) admin.Controller {
	return &saveController{config: c, env: env}
}

func (v *saveController) Execute(c *gin.Context) {
	var form formData
	if err := c.Bind(&form); err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	db := exportimportdatabase.New(v.env.Database())
	model := new(exportimportmodel.ExportImport)

	idRaw := c.Param("id")
	if idRaw != "" && idRaw != "0" {
		id, err := strconv.ParseInt(idRaw, 10, 64)
		if err != nil {
			admin.ErrorPage(c, "failed to to parse `id` param.")
			return
		}

		model, err = db.GetConfig(ctx, id)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("failed to load export importer config: %s", err))
			return
		}
	}

	if err := form.BuildExportImporterModel(model); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("failed to build export importer config: %s", err))
		return
	}

	fn := db.AddConfig
	if model.ID != 0 {
		fn = db.UpdateConfig
	}

	if err := fn(ctx, model); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("failed to write export importer config: %s", err))
		return
	}

	m.AddSuccess("Successfully updated export importer config!")
	m["model"] = model
	c.HTML(http.StatusOK, "export-importer", m)
	c.Abort()
}
