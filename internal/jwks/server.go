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
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/server"
	"github.com/gorilla/mux"
)

const jwksLock = "jwks-import"

// Server is the debugger server.
type Server struct {
	config  *Config
	manager *Manager
	env     *serverenv.ServerEnv
}

// NewServer makes a new debugger server.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("expected env to have database")
	}

	ctx := context.Background()
	manager, err := NewManager(ctx, env.Database(), config.KeyCleanupTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	return &Server{
		config:  config,
		manager: manager,
		env:     env,
	}, nil
}

func (s *Server) Routes(ctx context.Context) *mux.Router {
	r := mux.NewRouter()
	r.Handle("/health", server.HandleHealthz(ctx))
	r.Handle("/", s.handleUpdateAll(ctx))

	return r
}

func (s *Server) handleUpdateAll(ctx context.Context) http.Handler {
	logger := logging.FromContext(ctx).Named("handleUpdateAll")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		unlock, err := s.manager.db.LockRetry(ctx, jwksLock, time.Minute, s.config.Database.LockRetryTime)
		if err != nil {
			logger.Warnw("unable to acquire lock for jwks import", "error", err)
			if errors.Is(err, database.ErrAlreadyLocked) {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		defer func() {
			if err := unlock(); err != nil {
				logger.Errorw("failed to unlock", "error", err)
			}
		}()

		if err := s.manager.UpdateAll(ctx); err != nil {
			logger.Errorw("unable to import JWKS key sets", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, http.StatusText(http.StatusOK))
	})
}
