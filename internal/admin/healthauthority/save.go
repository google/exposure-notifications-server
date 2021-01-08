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

package healthauthority

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
)

type saveController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewSave(c *admin.Config, env *serverenv.ServerEnv) admin.Controller {
	return &saveController{config: c, env: env}
}

func (h *saveController) Execute(c *gin.Context) {
	var form formData
	err := c.Bind(&form)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	haDB := database.New(h.env.Database())
	healthAuthority := &model.HealthAuthority{}
	haID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		admin.ErrorPage(c, "Unable to parse `id` param")
		return
	}
	if haID != 0 {
		healthAuthority, err = haDB.GetHealthAuthorityByID(ctx, haID)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("error processing health authority: %v", err))
			return
		}
	}
	form.PopulateHealthAuthority(healthAuthority)

	// Decide if update or insert.
	updateFn := haDB.AddHealthAuthority
	if haID != 0 {
		updateFn = haDB.UpdateHealthAuthority
	}
	if err := updateFn(ctx, healthAuthority); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("Error writing health authority: %v", err))
		return
	}

	m.AddSuccess(fmt.Sprintf("Updated Health Authority '%v'", healthAuthority.Issuer))
	m["ha"] = healthAuthority
	m["hak"] = &model.HealthAuthorityKey{From: time.Now()} // For create form.
	c.HTML(http.StatusOK, "healthauthority", m)
}
