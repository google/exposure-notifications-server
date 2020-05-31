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

package authorizedapps

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
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

	aadb := database.New(h.env.Database())
	if form.Action == "save" {
		// Create new, or load previous.
		authApp := model.NewAuthorizedApp()
		priorKey := form.PriorKey()
		if priorKey != "" {
			authApp, err = aadb.GetAuthorizedApp(ctx, h.env.SecretManager(), priorKey)
			if err != nil {
				admin.ErrorPage(c, "Invalid request, app to edit not found.")
				return
			}
		}
		if err := form.PopulateAuthorizedApp(authApp); err != nil {
			admin.ErrorPage(c, err.Error())
			return
		}
		errors := authApp.Validate()
		if len(errors) > 0 {
			m.AddErrors(errors...)
			m["app"] = authApp
			c.HTML(http.StatusOK, "authorizedapp", m)
			return
		}

		if priorKey != "" {
			if err := aadb.UpdateAuthorizedApp(ctx, priorKey, authApp); err != nil {
				m.AddErrors(fmt.Sprintf("Error removing old version: %v", err))
				m["app"] = authApp
				c.HTML(http.StatusOK, "authorizedapp", m)
				return
			}
			m.AddSuccess(fmt.Sprintf("Updated authorized app: %v", authApp.AppPackageName))
		} else {
			if err := aadb.InsertAuthorizedApp(ctx, authApp); err != nil {
				m.AddErrors(fmt.Sprintf("Error inserting authorized app: %v", err))
			} else {
				m.AddSuccess(fmt.Sprintf("Saved authorized app: %v", authApp.AppPackageName))
			}
		}
		m["app"] = authApp
		m["previousKey"] = base64.StdEncoding.EncodeToString([]byte(authApp.AppPackageName))
		c.HTML(http.StatusOK, "authorizedapp", m)
		return
	} else if form.Action == "delete" {
		priorKey := form.PriorKey()

		log.Printf("Deleting authorized app: %v", priorKey)
		if err := aadb.DeleteAuthorizedApp(ctx, priorKey); err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error deleting authorized app: %v", err))
			return
		}

		m.AddSuccess(fmt.Sprintf("Successfully deleted app `%v`", priorKey))
		m["app"] = model.NewAuthorizedApp()
		c.HTML(http.StatusOK, "authorizedapp", m)
		return
	}

	admin.ErrorPage(c, "Invalid request, app to edit not found.")
}
