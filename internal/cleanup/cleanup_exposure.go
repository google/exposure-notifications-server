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

	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/gorilla/mux"
	"github.com/hashicorp/go-multierror"
	"go.opencensus.io/stats"
)

type ExposureServer struct {
	config   *Config
	env      *serverenv.ServerEnv
	database *database.PublishDB
}

// NewExposureServer creates a http.Handler for deleting exposure keys
// from the database.
func NewExposureServer(cfg *Config, env *serverenv.ServerEnv) (*ExposureServer, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	return &ExposureServer{
		config:   cfg,
		env:      env,
		database: database.New(env.Database()),
	}, nil
}

// Routes defines and returns the routes for the export server.
func (s *ExposureServer) Routes(ctx context.Context) *mux.Router {
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
func (s *ExposureServer) handleCleanup() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger := logging.FromContext(ctx).Named("cleanup.exposure")

		cutoff, err := cutoffDate(s.config.TTL, s.config.DebugOverrideCleanupMinDuration)
		if err != nil {
			logger.Errorw("failed to calculate cutoff date", "error", err)
			respondWithError(w, http.StatusInternalServerError, err)
			return
		}

		// Construct a multi-error. If one of the purges fails, we still want to
		// attempt the other purges.
		var merr *multierror.Error

		// Exposures
		func() {
			ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
			defer cancel()

			if count, err := s.database.DeleteExposuresBefore(ctx, cutoff); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to delete exposures: %w", err))
			} else {
				logger.Infow("purged exposures", "count", count)
			}
		}()

		// Stats
		func() {
			ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
			defer cancel()

			if count, err := s.database.DeleteStatsBefore(ctx, cutoff); err != nil {
				merr = multierror.Append(merr, fmt.Errorf("failed to delete stats: %w", err))
			} else {
				logger.Infow("purged statistics", "count", count)
			}
		}()

		if merr != nil {
			logger.Errorw("failed to cleanup exposures", "errors", merr.WrappedErrors())
			respondWithError(w, http.StatusInternalServerError, merr)
			return
		}

		stats.Record(ctx, mExposureSuccess.M(1))
		respond(w, http.StatusOK)
	})
}
