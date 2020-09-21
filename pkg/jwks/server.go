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
	"net/http"

	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
)

// Server is the debugger server.
type Server struct {
	config  *Config
	manager *Manager
	env     *serverenv.ServerEnv
}

// NewServer makes a new debugger server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	return &Server{
		config: config,
		env:    env,
	}, nil
}

func (s *Server) Routes(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/health", server.HandleHealthz(ctx))
	mux.Handle("/", s.handleUpdateAll(ctx))

	return mux
}

func (s *Server) handleUpdateAll(ctx context.Context) http.Handler {
	logger := logging.FromContext(ctx).Named("handleUpdateAll")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if err := s.manager.UpdateAll(ctx); err != nil {
			logger.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, http.StatusText(http.StatusOK))
	})
}
