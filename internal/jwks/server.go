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

package jwks

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"

	"github.com/gorilla/mux"
)

// Server is the server.
type Server struct {
	config  *Config
	manager *Manager
	env     *serverenv.ServerEnv
}

// NewServer makes a new server.
func NewServer(cfg *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server env")
	}

	manager, err := NewManager(env.Database(), cfg.KeyCleanupTTL, cfg.RequestTimeout, int64(cfg.MaxWorkers))
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	return &Server{
		config:  cfg,
		manager: manager,
		env:     env,
	}, nil
}

// Routes returns the router for this server.
func (s *Server) Routes(ctx context.Context) *mux.Router {
	logger := logging.FromContext(ctx).Named("jwks")

	r := mux.NewRouter()
	r.Use(middleware.PopulateRequestID())
	r.Use(middleware.PopulateObservability())
	r.Use(middleware.PopulateLogger(logger))

	r.Handle("/health", server.HandleHealthz(s.env.Database()))
	r.Handle("/", s.handleUpdateAll())

	return r
}
