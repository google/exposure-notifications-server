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

// Package index contains admin console indexHandler for the main landing page.
package index

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	aadb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	exdb "github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
)

type indexHandler struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func New(c *admin.Config, env *serverenv.ServerEnv) admin.Controller {
	return &indexHandler{config: c, env: env}
}

func (h *indexHandler) Execute(c *gin.Context) {
	ctx := c.Request.Context()
	m := admin.TemplateMap{}

	// Load authorized apps for index.
	db := aadb.New(h.env.Database())
	apps, err := db.ListAuthorizedApps(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["apps"] = apps

	// Load health authorities.
	haDB := hadb.New(h.env.Database())
	has, err := haDB.ListAllHealthAuthoritiesWithoutKeys(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["healthauthorities"] = has

	// Load export configurations.
	exportDB := exdb.New(h.env.Database())
	exports, err := exportDB.GetAllExportConfigs(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["exports"] = exports

	// Load SignatureInfos
	sigInfos, err := exportDB.ListAllSigntureInfos(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["siginfos"] = sigInfos

	m.AddTitle("Exposure Notifications Server - Admin Console")
	m.AddJumbotron("Exposure Notification Server", "Admin Console")
	c.HTML(http.StatusOK, "index", m)
}
