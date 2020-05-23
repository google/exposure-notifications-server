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

package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/authorizedapp/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

type appHandler struct {
	config *Config
	env    *serverenv.ServerEnv
}

func NewAppHandler(c *Config, env *serverenv.ServerEnv) *appHandler {
	return &appHandler{config: c, env: env}
}

func (h *appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		h.handleGet(w, r)
		return
	} else if r.Method == "POST" {
		h.handlePost(w, r)
		return
	}

	m := TemplateMap{}
	m.AddErrors("Invalid request.")
	h.config.RenderTemplate(w, "error", m)
}

func (h *appHandler) renderError(m TemplateMap, msg string, w http.ResponseWriter) {
	m.AddErrors(msg)
	h.config.RenderTemplate(w, "error", m)
}

func populateAuthorizedApp(a *model.AuthorizedApp, r *http.Request) []string {
	a.AppPackageName = r.FormValue("AppPackageName")
	a.Platform = r.FormValue("Platform")

	a.AllowedRegions = make(map[string]struct{})
	regions := strings.Split(r.FormValue("Regions"), "\n")
	for _, region := range regions {
		a.AllowedRegions[region] = struct{}{}
	}

	var err error
	errors := []string{}
	// SafetyNet pieces.
	a.SafetyNetDisabled, err = strconv.ParseBool(r.FormValue("SafetyNetDisabled"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("SafetyNetDisabled, invalid value: %v", err))
	}
	a.SafetyNetApkDigestSHA256 = strings.Split(r.FormValue("SafetyNetApkDigestSHA256"), "\n")
	a.SafetyNetBasicIntegrity, err = strconv.ParseBool(r.FormValue("SafetyNetBasicIntegrity"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("SafetyNetBasicIntegrity, invalid value: %v", err))
	}
	a.SafetyNetCTSProfileMatch, err = strconv.ParseBool(r.FormValue("SafetyNetCTSProfileMatch"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("SafetyNetCTSProfileMatch, invalid value: %v", err))
	}
	a.SafetyNetPastTime, err = time.ParseDuration(r.FormValue("SafetyNetPastTime"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("SafetyNetPastTime, invalid value: %v", err))
	}
	a.SafetyNetFutureTime, err = time.ParseDuration(r.FormValue("SafetyNetFutureTime"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("SafetyNetFutureTime, invalid value: %v", err))
	}

	// DeviceCheck pieces
	a.DeviceCheckDisabled, err = strconv.ParseBool(r.FormValue("DeviceCheckDisabled"))
	if err != nil {
		errors = append(errors, fmt.Sprintf("DeviceCheckDisabled, invalid value: %v", err))
	}
	a.DeviceCheckKeyID = r.FormValue("DeviceCheckKeyID")
	a.DeviceCheckTeamID = r.FormValue("DeviceCheckTeamID")
	a.DeviceCheckPrivateKeySecret = r.FormValue("DeviceCheckPrivateKeySecret")

	return errors
}

func decodePriorKey(priorKey string) (string, error) {
	if priorKey != "" {
		bytes, err := base64.StdEncoding.DecodeString(priorKey)
		if err != nil {
			return "", err
		}
		priorKey = string(bytes)
	}
	return priorKey, nil
}

func (h *appHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := TemplateMap{}
	err := r.ParseForm()
	if err != nil {
		defer h.renderError(m, "Invalid request", w)
		return
	}

	aadb := database.New(h.env.Database())

	if action := r.FormValue("TODO"); action == "save" {
		priorKey, err := decodePriorKey(r.FormValue("Key"))
		if err != nil {
			defer h.renderError(m, "Invalid request", w)
			return
		}

		// Create new, or load previous.
		authApp := model.NewAuthorizedApp()
		if priorKey != "" {
			authApp, err = aadb.GetAuthorizedApp(ctx, h.env.SecretManager(), priorKey)
			if err != nil {
				defer h.renderError(m, "Invalid request, app to edit not found.", w)
				return
			}
		}
		errors := populateAuthorizedApp(authApp, r)
		if len(errors) > 0 {
			m.AddErrors(errors...)
			m["app"] = authApp
			h.config.RenderTemplate(w, "app_view", m)
			return
		}

		errors = authApp.Validate()
		if len(errors) > 0 {
			m.AddErrors(errors...)
			m["app"] = authApp
			h.config.RenderTemplate(w, "app_view", m)
			return
		}

		if priorKey != "" {
			if err := aadb.DeleteAuthorizedApp(ctx, priorKey); err != nil {
				m.AddErrors(fmt.Sprintf("Error removing old version: %v", err))
				m["app"] = authApp
				h.config.RenderTemplate(w, "app_view", m)
				return
			}
		}

		if err := aadb.InsertAuthorizedApp(ctx, authApp); err != nil {
			m.AddErrors(fmt.Sprintf("Error inserting authorized app: %v", err))
		} else {
			m.AddSuccess(fmt.Sprintf("Saved authorized app: %v", authApp.AppPackageName))
		}
		m["app"] = authApp
		h.config.RenderTemplate(w, "app_view", m)
		return

	} else if action == "delete" {
		priorKey, err := decodePriorKey(r.FormValue("Key"))
		if err != nil {
			defer h.renderError(m, "Invalid request", w)
			return
		}

		_, err = aadb.GetAuthorizedApp(ctx, h.env.SecretManager(), priorKey)
		if err != nil {
			defer h.renderError(m, "Couldn't find record to delete.", w)
			return
		}

		if err := aadb.DeleteAuthorizedApp(ctx, priorKey); err != nil {
			defer h.renderError(m, fmt.Sprintf("Error deleting authorized app: %v", err), w)
			return
		}

		m.AddSuccess(fmt.Sprintf("Successfully deleted app `%v`", priorKey))
		m["app"] = model.NewAuthorizedApp()
		h.config.RenderTemplate(w, "app_view", m)
		return
	}

	h.renderError(m, "Invalid action requested", w)
}

func (h *appHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := TemplateMap{}

	appIds := r.URL.Query()["apn"]
	appID := ""
	if len(appIds) > 0 {
		appID = appIds[0]
	}

	authorizedApp := model.NewAuthorizedApp()

	if appID == "" {
		m.AddJumbotron("Authorized Applications", "Create New Authorized Application")
		m["new"] = true
	} else {
		aadb := database.New(h.env.Database())
		var err error
		authorizedApp, err = aadb.GetAuthorizedApp(ctx, h.env.SecretManager(), appID)
		if err != nil {
			m["error"] = err
			h.config.RenderTemplate(w, "error", m)
			return
		}
		m.AddJumbotron("Authorized Applications", fmt.Sprintf("Edit: `%v`", authorizedApp.AppPackageName))
	}
	m["app"] = authorizedApp
	m["previousKey"] = base64.StdEncoding.EncodeToString([]byte(authorizedApp.AppPackageName))
	h.config.RenderTemplate(w, "app_view", m)
}
