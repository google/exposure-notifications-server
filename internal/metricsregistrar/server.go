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

//go:generate go run ../../tools/gen-metrics-registrar -pkg metricsregistrar -dest ./all_metrics.go

package metricsregistrar

import (
	"context"
	"net/http"

	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/render"

	"github.com/gorilla/mux"
)

// Server is the server.
type Server struct {
	config *Config
	env    *serverenv.ServerEnv
	h      *render.Renderer
}

// NewServer makes a new server and registers the upstream metrics.
func NewServer(ctx context.Context, cfg *Config, env *serverenv.ServerEnv) (*Server, error) {
	srv := &Server{
		config: cfg,
		env:    env,
		h:      render.NewRenderer(),
	}

	if err := srv.createMetrics(ctx); err != nil {
		return nil, err
	}
	return srv, nil
}

// Routes returns the router for this server.
func (s *Server) Routes(ctx context.Context) *mux.Router {
	logger := logging.FromContext(ctx).Named("jwks")

	r := mux.NewRouter()
	r.Use(middleware.Recovery())
	r.Use(middleware.PopulateRequestID())
	r.Use(middleware.PopulateObservability())
	r.Use(middleware.PopulateLogger(logger))

	r.Handle("/", s.handleRoot())

	return r
}

func (s *Server) handleRoot() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.h.RenderJSON(w, http.StatusOK, nil)
	})
}
