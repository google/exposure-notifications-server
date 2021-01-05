// Copyright 2021 Google LLC
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

package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/maintenance"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"

	"github.com/mikehelmick/go-chaff"
)

func (h *PublishHandler) HandleMetrics() http.Handler {
	mResponder := maintenance.New(h.config)
	return h.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		mResponder.Handle(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx, span := trace.StartSpan(r.Context(), "(*publish.HandleMetrics)")
				defer span.End()

				var request verifyapi.MetricsRequest
				code, err := jsonutil.Unmarshal(w, r, &request)
				if err != nil {
					message := fmt.Sprintf("error unmarshalling API call, code: %v: %v", code, err)
					span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
					errorCode := verifyapi.ErrorBadRequest
					if code == http.StatusInternalServerError {
						errorCode = verifyapi.ErrorInternalError
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusBadRequest)
					fmt.Fprintf(w, "{\"error\": \"%v\", \"code\": \"%v\"}", err.Error(), errorCode)
					return
				}

				response, status := h.handleMetricsRequest(ctx, r.Header.Get("Authorization"), &request)

				if padding, err := generatePadding(h.config.StatsResponsePaddingMinBytes, h.config.StatsResponsePaddingRange); err != nil {
					logging.FromContext(ctx).Errorw("failed to pad response", "error", err)
				} else {
					response.Padding = padding
				}

				data, err := json.Marshal(response)
				if err != nil {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "{\"error\": \"%v\"}", err.Error())
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				fmt.Fprintf(w, "%s", data)
			})))
}

func (h *PublishHandler) handleMetricsRequest(ctx context.Context, bearerToken string, request *verifyapi.MetricsRequest) (*verifyapi.MetricsResponse, int) {
	logger := logging.FromContext(ctx)
	response := &verifyapi.MetricsResponse{}

	if !strings.HasPrefix(bearerToken, "Bearer ") {
		response.ErrorMessage = "Authorization header is not in `Bearer <token>` format"
		return response, http.StatusBadRequest
	}
	// Remove 'Bearer ' from the token.
	bearerToken = bearerToken[7:]

	// Validate JWT - if valid, the health authority ID (based on issuer) is returned.
	healthAuthorityID, err := h.verifier.AuthenticateStatsToken(ctx, bearerToken)
	if err != nil {
		response.ErrorMessage = err.Error()
		response.ErrorCode = verifyapi.ErrorUnauthorized
		return response, http.StatusUnauthorized
	}

	// retrieve stats
	stats, err := h.database.ReadStats(ctx, healthAuthorityID)
	if err != nil {
		logger.Errorw("error reading stats", "error", err)
		response.ErrorMessage = "error reading stats"
		response.ErrorCode = verifyapi.ErrorInternalError
		return response, http.StatusInternalServerError
	}

	// Nothing from the current hour can be shown.
	onlyBefore := time.Now().UTC().Truncate(time.Hour)

	// Combine days - this also filters things that are "too new" and days that don't meet the threshold.
	metricsDays := model.ReduceStats(stats, onlyBefore, h.config.StatsUploadMinimum)
	// Transform array of points to array of structs before return.
	response.Days = make([]verifyapi.MetricsDay, len(metricsDays))
	for i, day := range metricsDays {
		response.Days[i] = *day
	}

	// return
	return response, http.StatusOK
}
