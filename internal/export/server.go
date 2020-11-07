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
	"net/http"

	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/server"
)

// NewServer makes a Server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	// Validate config.
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires Blobstore present in the ServerEnv")
	}
	if env.Database() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires Database present in the ServerEnv")
	}
	if env.KeyManager() == nil {
		return nil, fmt.Errorf("export.NewBatchServer requires KeyManager present in the ServerEnv")
	}
	if config.MinWindowAge < 0 {
		return nil, fmt.Errorf("MIN_WINDOW_AGE must be a duration of >= 0")
	}

	return &Server{
		config: config,
		env:    env,
	}, nil
}

// Server hosts end points to manage export batches.
type Server struct {
	config *Config
	env    *serverenv.ServerEnv
}

// Routes defines and returns the routes for this server.
func (s *Server) Routes(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/create-batches", s.handleCreateBatches(ctx))
	mux.HandleFunc("/do-work", s.handleDoWork(ctx))
	mux.Handle("/health", server.HandleHealthz)

	return mux
}
