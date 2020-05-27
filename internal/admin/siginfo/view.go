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

package siginfo

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/export/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

type viewController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewView(c *admin.Config, env *serverenv.ServerEnv) *viewController {
	return &viewController{config: c, env: env}
}

func (v *viewController) Execute(c *gin.Context) {
	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	sigInfo := &model.SignatureInfo{}
	if IDParam := c.Param("id"); IDParam == "0" {
		m.AddJumbotron("Signature Info", "Create New Signature Info")
		m["new"] = true
	} else {
		m.AddJumbotron("Signature Info", "Edit Signature Info")

		exportDB := database.New(v.env.Database())
		sigID, err := strconv.ParseInt(IDParam, 10, 64)
		if err != nil {
			admin.ErrorPage(c, "Unable to parse `id` param.")
			return
		}
		sigInfo, err = exportDB.GetSignatureInfo(ctx, sigID)
		if err != nil {
			admin.ErrorPage(c, "error loading signtaure info.")
			return
		}
	}

	m["siginfo"] = sigInfo
	c.HTML(http.StatusOK, "siginfo", m)
}
