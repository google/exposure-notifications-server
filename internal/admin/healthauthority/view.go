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

type viewController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewView(c *admin.Config, env *serverenv.ServerEnv) admin.Controller {
	return &viewController{config: c, env: env}
}

func (v *viewController) Execute(c *gin.Context) {
	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	healthAuthority := &model.HealthAuthority{}
	if IDParam := c.Param("id"); IDParam == "0" {
		m.AddJumbotron("Health Authorities", "Create New Health Authority")
		m["new"] = true
	} else {
		m.AddJumbotron("Health Authorities", "Edit Health Authority")

		haID, err := strconv.ParseInt(IDParam, 10, 64)
		if err != nil {
			admin.ErrorPage(c, "Unable to parse `id` param.")
			return
		}

		haDB := database.New(v.env.Database())
		healthAuthority, err = haDB.GetHealthAuthorityByID(ctx, haID)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Unable to find requested health authority: %v. Error: %v", haID, err))
			return
		}
	}

	m["ha"] = healthAuthority
	m["hak"] = &model.HealthAuthorityKey{From: time.Now()} // For create form.
	c.HTML(http.StatusOK, "healthauthority", m)
}
