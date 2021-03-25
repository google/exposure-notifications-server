// Copyright 2021 Google LLC
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

package cleanup

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/export/database"
	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/render"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

type ExportServer struct {
	config    *Config
	env       *serverenv.ServerEnv
	database  *database.ExportDB
	blobstore storage.Blobstore
	h         *render.Renderer
}

// NewExportServer creates a server that manages deletion of old export files
// that are no longer needed by clients for download.
func NewExportServer(cfg *Config, env *serverenv.ServerEnv) (*ExportServer, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("missing blobstore in server environment")
	}

	return &ExportServer{
		config:    cfg,
		env:       env,
		database:  database.New(env.Database()),
		blobstore: env.Blobstore(),
		h:         render.NewRenderer(),
	}, nil
}

// Routes defines and returns the routes for the export server.
func (s *ExportServer) Routes(ctx context.Context) *mux.Router {
	logger := logging.FromContext(ctx).Named("mirror")

	r := mux.NewRouter()
	r.Use(middleware.Recovery())
	r.Use(middleware.PopulateRequestID())
	r.Use(middleware.PopulateObservability())
	r.Use(middleware.PopulateLogger(logger))

	r.Handle("/health", server.HandleHealthz(s.env.Database()))
	r.Handle("/", s.handleCleanup())

	return r
}

// handleCleanup handles export cleanup.
func (s *ExportServer) handleCleanup() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("cleanup.export")
		logger.Debugw("starting")
		defer logger.Debugw("finishing")

		cutoff, err := cutoffDate(s.config.TTL, s.config.DebugOverrideCleanupMinDuration)
		if err != nil {
			logger.Errorw("failed to calculate cutoff date", "error", err)
			s.h.RenderJSON(w, http.StatusInternalServerError, err)
			return
		}

		// Construct a multi-error. If one of the purges fails, we still want to
		// attempt the other purges.
		var merr *multierror.Error

		// Files
		func() {
			ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
			defer cancel()

			if count, err := s.database.DeleteFilesBefore(ctx, cutoff, s.blobstore); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to delete files: %w", err))
			} else {
				logger.Infow("purged files", "count", count)
			}
		}()

		if errs := merr.WrappedErrors(); len(errs) > 0 {
			logger.Errorw("failed to cleanup exports", "errors", errs)
			s.h.RenderJSON(w, http.StatusInternalServerError, errs)
			return
		}

		stats.Record(ctx, mExportSuccess.M(1))
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}
