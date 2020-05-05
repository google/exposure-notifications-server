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

// Package publish defines the exposure keys publishing API.
package publish

import (
	"context"
	"net/http"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/api/jsonutil"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/verification"
)

const (
	targetRequestDurationEnv = "TARGET_REQUEST_DURATION"
	defaultTargetDuration    = 5 * time.Second
)

// NewHandler creates the HTTP handler for the TTK publishing API.
func NewHandler(ctx context.Context, db *database.DB, cfg *config.Config) http.Handler {
	return &publishHandler{
		config: cfg,
		db:     db,
	}
}

type publishHandler struct {
	config *config.Config
	db     *database.DB
}

// There is a target normalized latency for this function. This is to help prevent
// clients from being able to distinguish from successful or errored requests.
func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)

	var data model.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		// Log the unparsable JSON, but return success to the client.
		logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
		w.WriteHeader(http.StatusOK)
		return
	}

	cfg, err := h.config.AppPkgConfig(ctx, data.AppPackageName)
	if err != nil {
		// Log the configuraiton error, return error to client.
		// This is retryable, although won't succede if the error isn't transient.
		logger.Errorf("no API config, dropping data: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		// configs were loaded, but the request app isn't configured.
		logger.Errorf("unauthorized application: %v", data.AppPackageName)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	err = verification.VerifyRegions(cfg, data)
	if err != nil {
		logger.Errorf("verification.VerifyRegions: %v", err)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	if cfg.IsIOS() {
		logger.Errorf("ios devicecheck not supported on this server.")
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	} else if cfg.IsAndroid() {
		err = verification.VerifySafetyNet(ctx, time.Now().UTC(), cfg, data)
		if err != nil {
			logger.Errorf("unable to verify safetynet payload: %v", err)
			// This returns success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}
	} else {
		logger.Errorf("invalid API configuration for AppPkg: %v, invalid platform", data.AppPackageName)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	batchTime := time.Now().UTC()
	exposures, err := model.TransformPublish(&data, batchTime)
	if err != nil {
		logger.Errorf("error transforming publish data: %v", err)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	err = h.db.InsertExposures(ctx, exposures)
	if err != nil {
		logger.Errorf("error writing exposure record: %v", err)
		// This is retryable at the client - database error at the server.
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	logger.Infof("Inserted %d exposures.", len(exposures))

	w.WriteHeader(http.StatusOK)
}
