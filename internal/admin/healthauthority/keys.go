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
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/admin"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification/database"
	"github.com/google/exposure-notifications-server/internal/verification/model"
)

type keyController struct {
	config *admin.Config
	env    *serverenv.ServerEnv
}

func NewKeyController(c *admin.Config, env *serverenv.ServerEnv) admin.Controller {
	return &keyController{config: c, env: env}
}

// Handles actions of : Create, Revoke, and Reinstate.
// Path of /healthauthoritykey/:id/:action/*version
func (h *keyController) Execute(c *gin.Context) {
	ctx := c.Request.Context()

	haDB := database.New(h.env.Database())
	haID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		admin.ErrorPage(c, "Unable to parse `id` param")
		return
	}
	healthAuthority, err := haDB.GetHealthAuthorityByID(ctx, haID)
	if err != nil {
		admin.ErrorPage(c, fmt.Sprintf("error processing health authority: %v", err))
		return
	}

	if action := c.Param("action"); action == "create" {
		var form keyFormData
		err := c.Bind(&form)
		if err != nil {
			admin.ErrorPage(c, err.Error())
			return
		}

		var hak model.HealthAuthorityKey
		err = form.PopulateHealthAuthorityKey(&hak)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error parsing new health authority key: %v", err))
			return
		}
		err = haDB.AddHealthAuthorityKey(ctx, healthAuthority, &hak)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error saving health authority key: %v", err))
			return
		}

	} else if action == "revoke" || action == "reinstate" || action == "activate" {
		version := c.Param("version")

		// find the key.
		var hak *model.HealthAuthorityKey
		for _, key := range healthAuthority.Keys {
			if key.Version == version {
				hak = key
				break
			}
		}
		if hak == nil {
			admin.ErrorPage(c, "Invalid key specified")
			return
		}

		if action == "activate" {
			if hak.IsFuture() {
				hak.From = time.Now()
			}
		} else if action == "revoke" {
			hak.Thru = time.Now()
			if !hak.Thru.After(hak.From) {
				// make it so that the key doesn't expire before it is active.
				hak.Thru = hak.From
			}
		} else {
			hak.Thru = time.Time{}
		}

		log.Printf("HAK %+v", hak)

		err = haDB.UpdateHealthAuthorityKey(ctx, hak)
		if err != nil {
			admin.ErrorPage(c, fmt.Sprintf("Error saving health authority key: %v", err))
			return
		}

	} else {
		admin.ErrorPage(c, "invalid action")
		return
	}
	c.Redirect(http.StatusFound, fmt.Sprintf("/healthauthority/%v", healthAuthority.ID))
}
