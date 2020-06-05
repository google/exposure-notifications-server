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
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/authorizedapp"
	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifydb "github.com/google/exposure-notifications-server/internal/verification/database"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
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

	transformer, err := model.NewTransformer(config.MaxKeysOnPublish, config.MaxIntervalAge, config.TruncateWindow, config.DebugReleaseSameDayKeys)
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
		database:              database.New(env.Database()),
		authorizedAppProvider: env.AuthorizedAppProvider(),
		verifier:              verification.New(verifydb.New(env.Database())),
	}, nil
}

type publishHandler struct {
	config                *Config
	serverenv             *serverenv.ServerEnv
	transformer           *model.Transformer
	database              *database.PublishDB
	authorizedAppProvider authorizedapp.Provider
	verifier              *verification.Verifier
}

type response struct {
	status      int
	message     string
	metric      string
	count       int // metricCount
	errorInProd bool
}

func (h *publishHandler) handleRequest(w http.ResponseWriter, r *http.Request) response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.publishHandler).handleRequest")
	defer span.End()

	logger := logging.FromContext(ctx)
	metrics := h.serverenv.MetricsExporter(ctx)

	var data verifyapi.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		// Log the unparsable JSON, but return success to the client.
		message := fmt.Sprintf("error unmarshalling API call, code: %v: %v", code, err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{status: http.StatusBadRequest, message: message, metric: "publish-bad-json", count: 1}
	}

	appConfig, err := h.authorizedAppProvider.AppConfig(ctx, data.AppPackageName)
	if err != nil {
		// Config loaded, but app with that name isn't registered. This can also
		// happen if the app was recently registered but the cache hasn't been
		// refreshed.
		if errors.Is(err, authorizedapp.ErrAppNotFound) {
			message := fmt.Sprintf("unauthorized app: %v", data.AppPackageName)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return response{status: http.StatusUnauthorized, message: message, metric: "publish-app-not-authorized", count: 1}
		}

		// A higher-level configuration error occurred, likely while trying to read
		// from the database. This is retryable, although won't succeed if the error
		// isn't transient.
		msg := fmt.Sprintf("no AuthorizedApp, dropping data: %v", err)
		logger.Error(msg)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: msg})
		return response{
			status:      http.StatusInternalServerError,
			message:     http.StatusText(http.StatusInternalServerError),
			metric:      "publish-error-loading-authorizedapp",
			count:       1,
			errorInProd: true,
		}
	}

	// Verify the request is from a permitted region.
	for _, r := range data.Regions {
		if !appConfig.IsAllowedRegion(r) {
			err := fmt.Errorf("app %v tried to write to unauthorized region %v", appConfig.AppPackageName, r)
			message := fmt.Sprintf("verifying allowed regions: %v", err)
			span.SetStatus(trace.Status{Code: trace.StatusCodePermissionDenied, Message: message})
			return response{
				status:  http.StatusUnauthorized,
				message: message,
				metric:  "publish-region-not-authorized",
				count:   1,
			}
		}
	}

	// Perform health authority certificat verification.
	overrides, err := h.verifier.VerifyDiagnosisCertificate(ctx, appConfig, &data)
	if err != nil {
		if appConfig.BypassHealthAuthorityVerification {
			logger.Warnf("bypassing health authority certificate verification for app: %v", appConfig.AppPackageName)
			metrics.WriteInt("publish-health-authority-verification-bypassed", true, 1)
		} else {
			message := fmt.Sprintf("unable to validate diagnosis verification: %v", err)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
			return response{status: http.StatusUnauthorized, message: message, metric: "publish-bad-verification", count: 1}
		}
	}

	// Apply overrides
	if len(overrides) > 0 {
		model.ApplyTransmissionRiskOverrides(&data, overrides)
	}

	batchTime := time.Now()
	exposures, err := h.transformer.TransformPublish(ctx, &data, batchTime)
	if err != nil {
		message := fmt.Sprintf("unable to read request data: %v", err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
		return response{status: http.StatusBadRequest, message: message, metric: "publish-transform-fail", count: 1}
	}

	err = h.database.InsertExposures(ctx, exposures)
	if err != nil {
		message := fmt.Sprintf("error writing exposure record: %v", err)
		logger.Error(message)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{status: http.StatusInternalServerError, message: http.StatusText(http.StatusInternalServerError), metric: "publish-db-write-error", count: 1}
	}

	message := fmt.Sprintf("Inserted %d exposures.", len(exposures))
	span.AddAttributes(trace.Int64Attribute("inserted_exposures", int64(len(exposures))))
	logger.Info(message)
	return response{
		status:  http.StatusOK,
		message: message,
		metric:  "publish-exposures-written",
		count:   len(exposures),
	}
}

// There is a target normalized latency for this function. This is to help prevent
// clients from being able to distinguish from successful or errored requests.
func (h *publishHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	response := h.handleRequest(w, r)

	if response.metric != "" {
		ctx := r.Context()
		metrics := h.serverenv.MetricsExporter(ctx)
		metrics.WriteInt(response.metric, true, response.count)
	}

	// Handle success. If debug enabled, write the message in the response.
	if response.status == http.StatusOK {
		if h.config.DebugAPIResponses {
			fmt.Fprint(w, response.message)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		return
	}

	// If this error is written in non-debug times or if debug is enabled, write
	// out the error and status.
	if h.config.DebugAPIResponses || response.errorInProd {
		w.WriteHeader(response.status)
		fmt.Fprint(w, response.message)
		return
	}

	// Normal production behaviour. Success it up.
	w.WriteHeader(http.StatusOK)
}
