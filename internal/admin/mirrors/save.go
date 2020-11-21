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

package mirrors

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	mirrordatabase "github.com/google/exposure-notifications-server/internal/mirror/database"
	mirrormodel "github.com/google/exposure-notifications-server/internal/mirror/model"
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

	db := mirrordatabase.New(v.env.Database())
	mirror := &mirrormodel.Mirror{}
	if idParam := c.Param("id"); idParam != "0" {
		cfgID, err := strconv.ParseInt(idParam, 10, 64)
		if err != nil {
			admin.ErrorPage(c, "unable to parse `id` param.")
			return
		}
		mirror, err = db.GetMirror(ctx, cfgID)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error loading mirror: %v", err))
			return
		}
	}

	switch form.Action {
	case "delete":
		if err := db.DeleteMirror(ctx, mirror); err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Failed to delete mirror: %v", err))
			return
		}

		m.AddSuccess(fmt.Sprintf("Deleted mirror %d", mirror.ID))
		c.Redirect(http.StatusSeeOther, "/")
	case "save":
		if err := form.PopulateMirror(mirror); err != nil {
			admin.ErrorPage(c, fmt.Sprintf("error processing mirror: %v", err))
			return
		}

		updateFn := db.AddMirror
		if mirror.ID != 0 {
			updateFn = db.UpdateMirror
		}
		if err := updateFn(ctx, mirror); err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error writing mirror: %v", err))
			return
		}

		m.AddSuccess(fmt.Sprintf("Updated mirror %d", mirror.ID))

		m["mirror"] = mirror
		c.HTML(http.StatusOK, "mirror", m)
	default:
		admin.ErrorPage(c, "Invalid form action")
	}
}
