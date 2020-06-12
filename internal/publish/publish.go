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
	exposuredatabase "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
)

type response struct {
	status      int
	message     string
	metric      string
	count       int // metricCount
	errorInProd bool
}

// handlerFunc is a handler that returns a response struct for further
// processing.
type handlerFunc func(w http.ResponseWriter, r *http.Request) *response

// handlePublish handles an exposure publication.
func (s *Server) handlePublish(ctx context.Context) handlerFunc {
	logger := logging.FromContext(ctx)
	metrics := s.env.MetricsExporter(ctx)
	db := s.env.Database()
	aaProvider := s.env.AuthorizedAppProvider()

	return func(w http.ResponseWriter, r *http.Request) *response {
		ctx, span := trace.StartSpan(r.Context(), "(*publish.publishHandler).handleRequest")
		defer span.End()

		var data verifyapi.Publish
		code, err := jsonutil.Unmarshal(w, r, &data)
		if err != nil {
			// Log the unparsable JSON, but return success to the client.
			message := fmt.Sprintf("error unmarshaling API call, code: %v: %v", code, err)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return &response{
				status:  http.StatusBadRequest,
				message: message,
				metric:  "publish-bad-json",
				count:   1,
			}
		}

		appConfig, err := aaProvider.AppConfig(ctx, data.AppPackageName)
		if err != nil {
			// Config loaded, but app with that name isn't registered. This can also
			// happen if the app was recently registered but the cache hasn't been
			// refreshed.
			if errors.Is(err, authorizedapp.ErrAppNotFound) {
				message := fmt.Sprintf("unauthorized app: %v", data.AppPackageName)
				logger.Error(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
				return &response{
					status:  http.StatusUnauthorized,
					message: message,
					metric:  "publish-app-not-authorized",
					count:   1,
				}
			}

			// A higher-level configuration error occurred, likely while trying to read
			// from the database. This is retryable, although won't succeed if the error
			// isn't transient.
			// This message (and logging) will only contain the AppPkgName from the request
			// and no other data from the request.
			msg := fmt.Sprintf("no AuthorizedApp, dropping data: %v", err)
			logger.Error(msg)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: msg})
			return &response{
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
				return &response{
					status:  http.StatusUnauthorized,
					message: message,
					metric:  "publish-region-not-authorized",
					count:   1,
				}
			}
		}

		// Perform health authority certificat verification.
		overrides, err := s.verifier.VerifyDiagnosisCertificate(ctx, appConfig, &data)
		if err != nil {
			if appConfig.BypassHealthAuthorityVerification {
				logger.Warnf("bypassing health authority certificate verification for app: %v", appConfig.AppPackageName)
				metrics.WriteInt("publish-health-authority-verification-bypassed", true, 1)
			} else {
				message := fmt.Sprintf("unable to validate diagnosis verification: %v", err)
				logger.Error(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
				return &response{
					status:  http.StatusUnauthorized,
					message: message,
					metric:  "publish-bad-verification",
					count:   1,
				}
			}
		}

		// Apply overrides
		if len(overrides) > 0 {
			publishmodel.ApplyTransmissionRiskOverrides(&data, overrides)
		}

		batchTime := time.Now()
		exposures, err := s.transformer.TransformPublish(ctx, &data, batchTime)
		if err != nil {
			message := fmt.Sprintf("unable to read request data: %v", err)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInvalidArgument, Message: message})
			return &response{
				status:  http.StatusBadRequest,
				message: message,
				metric:  "publish-transform-fail",
				count:   1,
			}
		}

		err = exposuredatabase.New(db).InsertExposures(ctx, exposures)
		if err != nil {
			message := fmt.Sprintf("error writing exposure record: %v", err)
			logger.Error(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			return &response{
				status:  http.StatusInternalServerError,
				message: http.StatusText(http.StatusInternalServerError),
				metric:  "publish-db-write-error",
				count:   1,
			}
		}

		message := fmt.Sprintf("Inserted %d exposures.", len(exposures))
		span.AddAttributes(trace.Int64Attribute("inserted_exposures", int64(len(exposures))))
		logger.Info(message)
		return &response{
			status:  http.StatusOK,
			message: message,
			metric:  "publish-exposures-written",
			count:   len(exposures),
		}
	}
}

// handleError handles the response from our custom handlerFunc.
func (s *Server) handleError(f handlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := f(w, r)

		if response.metric != "" {
			ctx := r.Context()
			metrics := s.env.MetricsExporter(ctx)
			metrics.WriteInt(response.metric, true, response.count)
		}

		// Handle success. If debug enabled, write the message in the response.
		if response.status == http.StatusOK {
			if s.config.DebugAPIResponses {
				fmt.Fprint(w, response.message)
			} else {
				w.WriteHeader(http.StatusOK)
			}
			return
		}

		// If this error is written in non-debug times or if debug is enabled, write
		// out the error and status.
		if s.config.DebugAPIResponses || response.errorInProd {
			w.WriteHeader(response.status)
			fmt.Fprint(w, response.message)
			return
		}

		// Normal production behavior. Success it up.
		w.WriteHeader(http.StatusOK)
	}
}
