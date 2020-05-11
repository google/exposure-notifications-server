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
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/exposure-notifications-server/internal/api/config"
	"github.com/google/exposure-notifications-server/internal/api/jsonutil"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
)

const (
	targetRequestDurationEnv = "TARGET_REQUEST_DURATION"
	defaultTargetDuration    = 5 * time.Second

	maxExposureKeysEnv       = "MAX_KEYS_ON_PUBLISH"
	maxExposureKeysDefault   = 14
	maxIntervalStartEnv      = "MAX_INTERVAL_AGE_ON_PUBLISH"
	maxIntervalStartDefault  = 15 * 24 * time.Hour // 15 days
	maxIntervalFutureEnv     = "MAX_INTERVAL_FUTURE_ON_PUBLISH"
	maxIntervalFutureDefault = 2 * time.Hour
)

// NewHandler creates the HTTP handler for the TTK publishing API.
func NewHandler(ctx context.Context, db *database.DB, cfg *config.Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx)

	maxKeys := maxExposureKeysDefault
	if val := os.Getenv(maxExposureKeysEnv); val != "" {
		parsed, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			logger.Errorf("couldn't parse env var %v, using defualt value %v", maxExposureKeysEnv, maxExposureKeysDefault)
		} else {
			maxKeys = int(parsed)
		}
	}
	maxAge := serverenv.ParseDuration(ctx, maxIntervalStartEnv, maxIntervalStartDefault)
	maxFuture := serverenv.ParseDuration(ctx, maxIntervalFutureEnv, maxIntervalFutureDefault)
	transformer, err := model.NewTransformer(maxKeys, maxAge, maxFuture)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}
	logger.Infof("max keys per upload: %v", maxKeys)
	logger.Infof("max interval start age: %v", maxAge)
	logger.Infof("max interval start future: %v", maxFuture)

	return &publishHandler{
		config:      cfg,
		db:          db,
		env:         env,
		transformer: transformer,
	}, nil
}

type publishHandler struct {
	config      *config.Config
	db          *database.DB
	env         *serverenv.ServerEnv
	transformer *model.Transformer
}

// There is a target normalized latency for this function. This is to help prevent
// clients from being able to distinguish from successful or errored requests.
func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	metrics := h.env.MetricsExporter(ctx)

	var data *model.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		// Log the unparsable JSON, but return success to the client.
		logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
		metrics.WriteInt("publish-bad-json", true, 1)
		w.WriteHeader(http.StatusOK)
		return
	}

	cfg, err := h.config.AppPkgConfig(ctx, data.AppPackageName)
	if err != nil {
		// Log the configuraiton error, return error to client.
		// This is retryable, although won't succede if the error isn't transient.
		logger.Errorf("no API config, dropping data: %v", err)
		metrics.WriteInt("publish-error-loading-apiconfig", true, 1)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if cfg == nil {
		// configs were loaded, but the request app isn't configured.
		logger.Errorf("unauthorized application: %v", data.AppPackageName)
		metrics.WriteInt("publish-app-not-authorized", true, 1)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	err = verification.VerifyRegions(cfg, data)
	if err != nil {
		logger.Errorf("verification.VerifyRegions: %v", err)
		metrics.WriteInt("publish-region-not-authorized", true, 1)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	if cfg.IsIOS() {
		if err := verification.VerifyDeviceCheck(ctx, cfg, data); err != nil {
			logger.Errorf("unable to verify devicecheck payload: %v", err)
			metrics.WriteInt("publish-devicecheck-invalid", true, 1)
			// Return success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}
	} else if cfg.IsAndroid() {
		if err := verification.VerifySafetyNet(ctx, time.Now(), cfg, data); err != nil {
			logger.Errorf("unable to verify safetynet payload: %v", err)
			metrics.WriteInt("publish-safetnet-invalid", true, 1)
			// Return success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}
	} else {
		logger.Errorf("invalid API configuration for AppPkg: %v, invalid platform", data.AppPackageName)
		metrics.WriteInt("publish-apiconfig-missing-platform", true, 1)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	batchTime := time.Now()
	exposures, err := h.transformer.TransformPublish(data, batchTime)
	if err != nil {
		logger.Errorf("error transforming publish data: %v", err)
		metrics.WriteInt("publish-transform-fail", true, 1)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	err = h.db.InsertExposures(ctx, exposures)
	if err != nil {
		logger.Errorf("error writing exposure record: %v", err)
		metrics.WriteInt("publish-db-write-error", true, 1)
		// This is retryable at the client - database error at the server.
		http.Error(w, "internal processing error", http.StatusInternalServerError)
		return
	}
	metrics.WriteInt("publish-exposures-written", true, len(exposures))
	logger.Infof("Inserted %d exposures.", len(exposures))

	w.WriteHeader(http.StatusOK)
}
