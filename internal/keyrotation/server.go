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

// Package keyrotation implements the API handlers for running key rotation jobs.
package keyrotation

import (
	"context"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/logging"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

// NewServer creates a Server that manages deletion of
// old export files that are no longer needed by clients for download.
func NewServer(config *Config, env *serverenv.ServerEnv) (*Server, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.GetKeyManager() == nil {
		return nil, fmt.Errorf("missing key manager in server environment")
	}

	revisionKeyConfig := revisiondb.KMSConfig{
		WrapperKeyID: config.RevisionToken.KeyID,
		KeyManager:   env.GetKeyManager(),
	}
	revisionDB, err := revisiondb.New(env.Database(), &revisionKeyConfig)
	if err != nil {
		return nil, fmt.Errorf("revisiondb.New: %w", err)
	}

	return &Server{
		config:     config,
		env:        env,
		revisionDB: revisionDB,
	}, nil
}

// Server hosts end points to manage key rotation
type Server struct {
	config     *Config
	env        *serverenv.ServerEnv
	revisionDB *revisiondb.RevisionDB
}

// Routes defines and returns the routes for this server.
func (s *Server) Routes(ctx context.Context) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/rotate-keys", s.handleRotateKeys(ctx))

	return mux
}

func (s *Server) handleRotateKeys(ctx context.Context) http.HandlerFunc {
	logger := logging.FromContext(ctx).Named("keyrotation.HandleRotate")

	return func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
		defer span.End()

		// TODO(whaught):
		// 1. Retrieve keys
		// 2. Early exit if the newest is new (configurable)
		// 3. Take a lock on the DB
		// 4. Otherwise generate a key and store
		// 5. delete oldest keys, but always keep any within 15d or still primary
		// 6. Metric on count created and deleted
		// 7. logger.log that too
		// 8. Unlock

		logger.Info("key rotation complete.")
		w.WriteHeader(http.StatusOK)
	}
}
