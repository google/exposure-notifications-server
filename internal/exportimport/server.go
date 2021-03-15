// Copyright 2020 Google LLC
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

// Package exportimport implements the handlers for the export-importer functionality.
package exportimport

import (
	"context"
	"fmt"

	eidb "github.com/google/exposure-notifications-server/internal/exportimport/database"
	"github.com/google/exposure-notifications-server/internal/middleware"
	pubdb "github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/database"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/gorilla/mux"
)

// Server hosts end points to manage key rotation
type Server struct {
	config         *Config
	env            *serverenv.ServerEnv
	db             *database.DB
	exportImportDB *eidb.ExportImportDB
	publishDB      *pubdb.PublishDB
}

// NewServer creates a Server that manages deletion of
// old export files that are no longer needed by clients for download.
func NewServer(cfg *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	if rt := cfg.BackfillReportType; !(rt == "" || rt == verifyapi.ReportTypeConfirmed || rt == verifyapi.ReportTypeClinical) {
		return nil, fmt.Errorf("BACKFILL_REPORT_TYPE value is invalid, must be %q, %q, or %q", "", verifyapi.ReportTypeConfirmed, verifyapi.ReportTypeClinical)
	}

	db := env.Database()
	exportImportDB := eidb.New(db)
	publishDB := pubdb.New(db)

	return &Server{
		config:         cfg,
		env:            env,
		db:             db,
		exportImportDB: exportImportDB,
		publishDB:      publishDB,
	}, nil
}

// Routes defines and returns the routes for this server.
func (s *Server) Routes(ctx context.Context) *mux.Router {
	logger := logging.FromContext(ctx).Named("exportimport")

	r := mux.NewRouter()
	r.Use(middleware.PopulateRequestID())
	r.Use(middleware.PopulateObservability())
	r.Use(middleware.PopulateLogger(logger))

	r.Handle("/health", server.HandleHealthz(s.env.Database()))
	r.Handle("/schedule", s.handleSchedule())
	r.Handle("/import", s.handleImport())

	return r
}
