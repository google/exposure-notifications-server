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
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/logging"
	revisiondb "github.com/google/exposure-notifications-server/internal/revision/database"
	"github.com/google/exposure-notifications-server/internal/serverenv"
)

// NewRotationHandler creates a http.Handler that manages deletion of
// old export files that are no longer needed by clients for download.
func NewRotationHandler(config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
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

	return &handler{
		config:     config,
		env:        env,
		revisionDB: revisionDB,
	}, nil
}

type handler struct {
	config     *Config
	env        *serverenv.ServerEnv
	revisionDB *revisiondb.RevisionDB
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*keyrotation.handler).ServeHTTP")
	defer span.End()

	logger := logging.FromContext(ctx)

	// TODO(whaught):
	// 1. Retrieve keys
	// 2. Early exit if the newest is new (configurable)
	// 2. Otherwise generate a key and store
	// 3. delete oldest keys, but always keep recent and min 2
	// 4. Metric on count created and deleted
	// 5. logger.log that too

	logger.Info("key rotation complete.")
	w.WriteHeader(http.StatusOK)
}
