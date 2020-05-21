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
	"net/http"

	authorizedappdb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
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
	ctx := r.Context()
	m := map[string]interface{}{}

	appIds := r.URL.Query()["apn"]
	appID := ""
	if len(appIds) > 0 {
		appID = appIds[0]
	}

	authorizedApp := model.NewAuthorizedApp()

	if appID == "" {
		m["new"] = true
	} else {
		aadb := authorizedappdb.NewAuthorizedAppDB(h.env.Database())
		var err error
		authorizedApp, err = aadb.GetAuthorizedApp(ctx, h.env.SecretManager(), appID)
		if err != nil {
			m["error"] = err
			h.config.RenderTemplate(w, "error", &m)
			return
		}
	}
	m["app"] = authorizedApp
	h.config.RenderTemplate(w, "app_view", &m)
}
