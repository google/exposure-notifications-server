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
	exportimportdatabase "github.com/google/exposure-notifications-server/internal/exportimport/database"
	mirrordatabase "github.com/google/exposure-notifications-server/internal/mirror/database"
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

	db := h.env.Database()

	// Load authorized apps for index.
	apps, err := aadb.New(db).ListAuthorizedApps(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["apps"] = apps

	// Load health authorities.
	has, err := hadb.New(db).ListAllHealthAuthoritiesWithoutKeys(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["healthauthorities"] = has

	// Load export configurations.
	exports, err := exdb.New(db).GetAllExportConfigs(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["exports"] = exports

	// Load export importer configurations.
	exportImporters, err := exportimportdatabase.New(db).ListConfigs(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["exportImporters"] = exportImporters

	// Load SignatureInfos
	sigInfos, err := exdb.New(db).ListAllSignatureInfos(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["siginfos"] = sigInfos

	// Load mirrors
	mirrors, err := mirrordatabase.New(db).Mirrors(ctx)
	if err != nil {
		admin.ErrorPage(c, err.Error())
		return
	}
	m["mirrors"] = mirrors

	m.AddTitle("Exposure Notification Key Server - Admin Console")
	c.HTML(http.StatusOK, "index", m)
}
