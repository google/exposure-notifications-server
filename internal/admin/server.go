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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

// Server is the admin server.
type Server struct {
	config *Config
	env    *serverenv.ServerEnv
}

// NewServer makes a new admin console server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing Database in server env")
	}

	return &Server{
		config: config,
		env:    env,
	}, nil
}

func (s *Server) Routes(ctx context.Context) http.Handler {
	tmpl, err := s.config.TemplateRenderer()
	if err != nil {
		panic(fmt.Errorf("failed to load templates: %w", err))
	}

	mux := gin.Default()
	mux.SetFuncMap(TemplateFuncMap)
	mux.SetHTMLTemplate(tmpl)

	// Landing page.
	mux.GET("/", s.HandleIndex())

	// Authorized App Handling.
	mux.GET("/app", s.HandleAuthorizedAppsShow())
	mux.POST("/app", s.HandleAuthorizedAppsSave())

	// HealthAuthority[Key] Handling.
	mux.GET("/healthauthority/:id", s.HandleHealthAuthorityShow())
	mux.POST("/healthauthority/:id", s.HandleHealthAuthoritySave())
	mux.POST("/healthauthoritykey/:id/:action/:version", s.HandleHealthAuthorityKeys())

	// Export Config Handling.
	mux.GET("/exports/:id", s.HandleExportsShow())
	mux.POST("/exports/:id", s.HandleExportsSave())

	// Export importer configuration
	mux.GET("/export-importers/:id", s.HandleExportImportersShow())
	mux.POST("/export-importers/:id", s.HandleExportImportersSave())
	mux.POST("/export-importers-key/:id/:action/:keyid", s.HandleExportImportKeys())

	// Mirror handling.
	mux.GET("/mirrors/:id", s.HandleMirrorsShow())
	mux.POST("/mirrors/:id", s.HandleMirrorsSave())

	// Signature Info.
	mux.GET("/siginfo/:id", s.HandleSignatureInfosShow())
	mux.POST("/siginfo/:id", s.HandleSignatureInfosSave())

	// Healthz.
	mux.GET("/health", s.HandleHealthz())

	return mux
}
