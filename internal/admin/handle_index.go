// Copyright 2021 Google LLC
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

package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	aadb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	exdb "github.com/google/exposure-notifications-server/internal/export/database"
	exportimportdatabase "github.com/google/exposure-notifications-server/internal/exportimport/database"
	mirrordatabase "github.com/google/exposure-notifications-server/internal/mirror/database"
	hadb "github.com/google/exposure-notifications-server/internal/verification/database"
)

func (s *Server) HandleIndex() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		db := s.env.Database()

		// Load authorized apps for index.
		apps, err := aadb.New(db).ListAuthorizedApps(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["apps"] = apps

		// Load health authorities.
		has, err := hadb.New(db).ListAllHealthAuthoritiesWithoutKeys(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["healthauthorities"] = has

		// Load export configurations.
		exports, err := exdb.New(db).GetAllExportConfigs(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["exports"] = exports

		// Load export importer configurations.
		exportImporters, err := exportimportdatabase.New(db).ListConfigs(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["exportImporters"] = exportImporters

		// Load SignatureInfos
		sigInfos, err := exdb.New(db).ListAllSignatureInfos(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["siginfos"] = sigInfos

		// Load mirrors
		mirrors, err := mirrordatabase.New(db).Mirrors(ctx)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}
		m["mirrors"] = mirrors

		m.AddTitle("Exposure Notification Key Server - Admin Console")
		c.HTML(http.StatusOK, "index", m)
	}
}
