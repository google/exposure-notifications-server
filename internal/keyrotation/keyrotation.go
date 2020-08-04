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

	"github.com/google/exposure-notifications-server/internal/export/database"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/storage"
)

// NewRotationHandler creates a http.Handler that manages deletion of
// old export files that are no longer needed by clients for download.
func NewRotationHandler(config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.Blobstore() == nil {
		return nil, fmt.Errorf("missing blobstore in server environment")
	}

	return &handler{
		config:    config,
		env:       env,
		database:  database.New(env.Database()),
		blobstore: env.Blobstore(),
	}, nil
}

type handler struct {
	config    *Config
	env       *serverenv.ServerEnv
	database  *database.ExportDB
	blobstore storage.Blobstore
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
