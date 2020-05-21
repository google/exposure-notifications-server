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

	aadb "github.com/google/exposure-notifications-server/internal/authorizedapp/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

type indexHandler struct {
	config *Config
	env    *serverenv.ServerEnv
}

func NewIndexHandler(c *Config, env *serverenv.ServerEnv) *indexHandler {
	return &indexHandler{config: c, env: env}
}

func (h *indexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := map[string]interface{}{}

	// Load authorized apps for index.
	db := aadb.NewAuthorizedAppDB(h.env.Database())
	apps, err := db.GetAllAuthorizedApps(ctx, h.env.SecretManager())
	if err != nil {
		m["error"] = err
		h.config.RenderTemplate(w, "error", &m)
		return
	}
	m["apps"] = apps

	// Load export configurations.
	exports, err := h.env.Database().GetAllExportConfigs(ctx)
	if err != nil {
		m["error"] = err
		h.config.RenderTemplate(w, "error", &m)
		return
	}
	m["exports"] = exports

	h.config.RenderTemplate(w, "index", &m)
}
