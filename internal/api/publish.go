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

// Package api defines the structures for the infection publishing API.
package api

import (
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/verification"
)

// NewPublishHandler creates the HTTP handler for the TTK publishing API.
func NewPublishHandler(db *database.DB, cfg *config.Config) http.Handler {
	return &publishHandler{
		config: cfg,
		db:     db,
	}
}

type publishHandler struct {
	config *config.Config
	db     *database.DB
}

func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	var data model.Publish
	err, code := unmarshal(w, r, &data)
	if err != nil {
		logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
		// Log but don't return internal decode error message reason.
		http.Error(w, "bad API request", code)
		return
	}

	cfg := h.config.AppPkgConfig(ctx, data.AppPackageName)
	if cfg == nil {
		// configs were loaded, but the request app isn't configured.
		logger.Errorf("unauthorized applicaiton: %v", data.AppPackageName)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	err = verification.VerifyRegions(cfg, data)
	if err != nil {
		logger.Errorf("verification.VerifyRegions: %v", err)
		// TODO(mikehelmick) change error code after clients verify functionality.
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	requestTime := time.Now().UTC()
	err = verification.VerifySafetyNet(ctx, requestTime, cfg, data)
	if err != nil {
		logger.Errorf("unable to verify safetynet payload: %v", err)
		// TODO(mikehelmick) change error code after clients verify functionality.
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	batchTime := time.Now().UTC()
	infections, err := model.TransformPublish(&data, batchTime)
	if err != nil {
		logger.Errorf("error transforming publish data: %v", err)
		http.Error(w, "bad API request", http.StatusBadRequest)
		return
	}

	err = h.db.InsertInfections(ctx, infections)
	if err != nil {
		logger.Errorf("error writing infection record: %v", err)
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Inserted %d infections.", len(infections))

	w.WriteHeader(http.StatusOK)
}
