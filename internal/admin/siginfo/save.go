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

func (h *saveController) Execute(c *gin.Context) {
	var form formData
	err := c.Bind(&form)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}

	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	exportDB := database.New(h.env.Database())

	sigID, err := strconv.Atoi(c.Param("id"))
	sigInfo := &model.SignatureInfo{}
	if err != nil {
		admin.ErrorPage(c, "Unable to parse `id` param.")
		return
	}
	if sigID != 0 {
		sigInfo, err = exportDB.GetSignatureInfo(ctx, int64(sigID))
	}
	if err := form.PopulateSigInfo(sigInfo); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("error processing signature info: %v", err))
		return
	}

	// Either insert or update.
	updateFn := exportDB.AddSignatureInfo
	if sigID != 0 {
		updateFn = exportDB.UpdateSignatureInfo
	}
	if err := updateFn(ctx, sigInfo); err != nil {
		admin.ErrorPage(c, fmt.Sprintf("Error writing signature info: %v", err))
		return
	}

	m.AddSuccess(fmt.Sprintf("Updated signture info #`%v`", sigInfo.ID))
	m["siginfo"] = sigInfo
	c.HTML(http.StatusOK, "siginfo", m)
}
