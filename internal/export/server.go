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

package export

import (
	"context"
	"fmt"

	"github.com/google/exposure-notifications-server/internal/middleware"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/gorilla/mux"
)

// NewServer makes a Server.
func NewServer(cfg *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires Blobstore present in the ServerEnv")
	}
	if env.Database() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires Database present in the ServerEnv")
	}
	if env.KeyManager() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires KeyManager present in the ServerEnv")
	}
	if cfg.MinWindowAge < 0 {
		return nil, fmt.Errorf("MIN_WINDOW_AGE must be a duration of >= 0")
	}

	return &Server{
		config: cfg,
		env:    env,
	}, nil
}

// Server hosts end points to manage export batches.
type Server struct {
	config *Config
	env    *serverenv.ServerEnv
}

// Routes defines and returns the routes for this server.
func (s *Server) Routes(ctx context.Context) *mux.Router {
	logger := logging.FromContext(ctx).Named("export")

	r := mux.NewRouter()
	r.Use(middleware.Recovery())
	r.Use(middleware.PopulateRequestID())
	r.Use(middleware.PopulateObservability())
	r.Use(middleware.PopulateLogger(logger))

	r.Handle("/health", server.HandleHealthz(s.env.Database()))
	r.Handle("/create-batches", s.handleCreateBatches())
	r.Handle("/do-work", s.handleDoWork())

	return r
}
