// Copyright 2021 the Exposure Notifications Server authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/project"
	verdb "github.com/google/exposure-notifications-server/internal/verification/database"
)

// HandleAuthorizedAppsSave handles the create/update actions for authorized
// apps.
func (s *Server) HandleAuthorizedAppsSave() func(c *gin.Context) {
	return func(c *gin.Context) {
		var form authorizedAppFormData
		err := c.Bind(&form)
		if err != nil {
			ErrorPage(c, err.Error())
			return
		}

		ctx := c.Request.Context()
		m := TemplateMap{}

		aadb := database.New(s.env.Database())
		if form.Action == "save" {
			// Create new, or load previous.
			authApp := model.NewAuthorizedApp()
			priorKey := form.PriorKey()
			if priorKey != "" {
				authApp, err = aadb.GetAuthorizedApp(ctx, priorKey)
				if err != nil {
					ErrorPage(c, "Invalid request, app to edit not found.")
					return
				}
				if authApp == nil {
					ErrorPage(c, "Unknown authorized app")
					return
				}
			}

			form.PopulateAuthorizedApp(authApp)

			errors := authApp.Validate()
			if len(errors) > 0 {
				m.AddErrors(errors...)
				m["app"] = authApp
				// Load the health authorities into m for display on the edit form.
				if err := addHealthAuthorityInfo(ctx, verdb.New(s.env.Database()), authApp, m); err != nil {
					ErrorPage(c, err.Error())
					return
				}
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

			if err := addHealthAuthorityInfo(ctx, verdb.New(s.env.Database()), authApp, m); err != nil {
				ErrorPage(c, err.Error())
				return
			}

			m["app"] = authApp
			m["previousKey"] = base64.StdEncoding.EncodeToString([]byte(authApp.AppPackageName))
			c.HTML(http.StatusOK, "authorizedapp", m)
			return
		} else if form.Action == "delete" {
			priorKey := form.PriorKey()

			if err := aadb.DeleteAuthorizedApp(ctx, priorKey); err != nil {
				ErrorPage(c, fmt.Sprintf("Error deleting authorized app: %v", err))
				return
			}

			authorizedApp := model.NewAuthorizedApp()
			if err := addHealthAuthorityInfo(ctx, verdb.New(s.env.Database()), authorizedApp, m); err != nil {
				ErrorPage(c, err.Error())
				return
			}

			m.AddSuccess(fmt.Sprintf("Successfully deleted app `%v`", priorKey))
			m["app"] = authorizedApp
			c.HTML(http.StatusOK, "authorizedapp", m)
			return
		}

		ErrorPage(c, "Invalid request, app to edit not found.")
	}
}

// HandleAuthorizedAppsShow handles the show page for authorized apps.
func (s *Server) HandleAuthorizedAppsShow() func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		m := TemplateMap{}

		appID, _ := c.GetQuery("apn")
		authorizedApp := model.NewAuthorizedApp()

		if appID == "" {
			m["new"] = true
		} else {
			aadb := database.New(s.env.Database())
			var err error
			authorizedApp, err = aadb.GetAuthorizedApp(ctx, appID)
			if err != nil {
				ErrorPage(c, err.Error())
				return
			}
			if authorizedApp == nil {
				ErrorPage(c, "error loading authorized app")
				return
			}
		}

		// Load the health authorities.
		if err := addHealthAuthorityInfo(ctx, verdb.New(s.env.Database()), authorizedApp, m); err != nil {
			ErrorPage(c, err.Error())
			return
		}

		m["app"] = authorizedApp
		m["previousKey"] = base64.StdEncoding.EncodeToString([]byte(authorizedApp.AppPackageName))
		c.HTML(http.StatusOK, "authorizedapp", m)
	}
}

// addHealthAuthorityInfo is a helper that adds HA info to the template map.
func addHealthAuthorityInfo(ctx context.Context, haDB *verdb.HealthAuthorityDB, app *model.AuthorizedApp, m TemplateMap) error {
	// Load the health authorities.
	healthAuthorities, err := haDB.ListAllHealthAuthoritiesWithoutKeys(ctx)
	if err != nil {
		return fmt.Errorf("error loading health authorities: %w", err)
	}

	usedAuthorities := make(map[int64]bool)
	for _, k := range app.AllAllowedHealthAuthorityIDs() {
		usedAuthorities[k] = true
	}

	m["has"] = healthAuthorities
	m["usedHealthAuthorities"] = usedAuthorities
	return nil
}

type authorizedAppFormData struct {
	// Top Level
	FormKey string `form:"key"`
	Action  string `form:"action"`

	// Authorized App Data
	AppPackageName                    string  `form:"app-package-name"`
	AllowedRegions                    string  `form:"regions"`
	BypassHealthAuthorityVerification bool    `form:"bypass-health-authority-verification"`
	BypassRevisionToken               bool    `form:"bypass-revision-token"`
	HealthAuthorityIDs                []int64 `form:"health-authorities"`
}

func (f *authorizedAppFormData) PriorKey() string {
	if f.FormKey == "" {
		return ""
	}

	bytes, err := base64.StdEncoding.DecodeString(f.FormKey)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func (f *authorizedAppFormData) PopulateAuthorizedApp(a *model.AuthorizedApp) {
	a.AppPackageName = project.TrimSpaceAndNonPrintable(f.AppPackageName)
	a.AllowedRegions = make(map[string]struct{})
	for _, region := range strings.Split(f.AllowedRegions, "\n") {
		region = project.TrimSpaceAndNonPrintable(region)
		if region != "" {
			a.AllowedRegions[project.TrimSpaceAndNonPrintable(region)] = struct{}{}
		}
	}
	a.AllowedHealthAuthorityIDs = make(map[int64]struct{})
	for _, haID := range f.HealthAuthorityIDs {
		a.AllowedHealthAuthorityIDs[haID] = struct{}{}
	}
	a.BypassHealthAuthorityVerification = f.BypassHealthAuthorityVerification
	a.BypassRevisionToken = f.BypassRevisionToken
}
