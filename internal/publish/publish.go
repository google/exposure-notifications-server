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
	"time"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/database"
	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
)

// NewHandler creates the HTTP handler for the TTK publishing API.
func NewHandler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx)

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}
	if env.AuthorizedAppProvider() == nil {
		return nil, fmt.Errorf("missing AuthorizedApp provider in server environment")
	}

	transformer, err := model.NewTransformer(config.MaxKeysOnPublish, config.MaxIntervalAge, config.TruncateWindow)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}
	logger.Infof("max keys per upload: %v", config.MaxKeysOnPublish)
	logger.Infof("max interval start age: %v", config.MaxIntervalAge)
	logger.Infof("truncate window: %v", config.TruncateWindow)

	return &publishHandler{
		serverenv:             env,
		transformer:           transformer,
		config:                config,
		database:              env.Database(),
		authorizedAppProvider: env.AuthorizedAppProvider(),
	}, nil
}

type publishHandler struct {
	config                *Config
	serverenv             *serverenv.ServerEnv
	transformer           *model.Transformer
	database              *database.DB
	authorizedAppProvider authorizedapp.Provider
}

// There is a target normalized latency for this function. This is to help prevent
// clients from being able to distinguish from successful or errored requests.
func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logging.FromContext(ctx)
	metrics := h.serverenv.MetricsExporter(ctx)

	var data *model.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		// Log the unparsable JSON, but return success to the client.
		logger.Errorf("error unmarhsaling API call, code: %v: %v", code, err)
		metrics.WriteInt("publish-bad-json", true, 1)
		w.WriteHeader(http.StatusOK)
		return
	}

	appConfig, err := h.authorizedAppProvider.AppConfig(ctx, data.AppPackageName)
	if err != nil {
		// Config loaded, but app with that name isn't registered. This can also
		// happen if the app was recently registered but the cache hasn't been
		// refreshed.
		if err == authorizedapp.AppNotFound {
			logger.Errorf("unauthorized app: %v", data.AppPackageName)
			metrics.WriteInt("publish-app-not-authorized", true, 1)
			// This returns success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}

		// A higher-level configuration error occurred, likely while trying to read
		// from the database. This is retryable, although won't succeed if the error
		// isn't transient.
		logger.Errorf("no AuthorizedApp, dropping data: %v", err)
		metrics.WriteInt("publish-error-loading-authorizedapp", true, 1)
		http.Error(w, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}

	if err := verification.VerifyRegions(appConfig, data); err != nil {
		logger.Errorf("verification.VerifyRegions: %v", err)
		metrics.WriteInt("publish-region-not-authorized", true, 1)
		// This returns success to the client.
		w.WriteHeader(http.StatusOK)
		return
	}

	if appConfig.IsIOS() {
		if h.config.BypassDeviceCheck {
			logger.Errorf("bypassing DeviceCheck for %v", data.AppPackageName)
			metrics.WriteInt("publish-devicecheck-bypass", true, 1)
		} else if err := verification.VerifyDeviceCheck(ctx, appConfig, data); err != nil {
			logger.Errorf("unable to verify devicecheck payload: %v", err)
			metrics.WriteInt("publish-devicecheck-invalid", true, 1)
			// Return success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}
	} else if appConfig.IsAndroid() {
		if h.config.BypassSafetyNet {
			logger.Errorf("bypassing SafetyNet for %v", data.AppPackageName)
			metrics.WriteInt("publish-safetynet-bypass", true, 1)
		} else if err := verification.VerifySafetyNet(ctx, time.Now(), appConfig, data); err != nil {
			logger.Errorf("unable to verify safetynet payload: %v", err)
			metrics.WriteInt("publish-safetnet-invalid", true, 1)
			// Return success to the client.
			w.WriteHeader(http.StatusOK)
			return
		}
	} else {
		logger.Errorf("invalid AuthorizedApp config %v: invalid platform %v",
			data.AppPackageName, data.Platform)
		metrics.WriteInt("publish-authorizedapp-missing-platform", true, 1)
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

	err = h.database.InsertExposures(ctx, exposures)
	if err != nil {
		logger.Errorf("error writing exposure record: %v", err)
		metrics.WriteInt("publish-db-write-error", true, 1)
		// This is retryable at the client - database error at the server.
		http.Error(w, http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError)
		return
	}
	metrics.WriteInt("publish-exposures-written", true, len(exposures))
	logger.Infof("Inserted %d exposures.", len(exposures))

	w.WriteHeader(http.StatusOK)
}
