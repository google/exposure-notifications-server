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
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/trace"
)

func (s *Server) handleStats() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.StartSpan(r.Context(), "(*publish.HandleStats)")
		defer span.End()

		var request verifyapi.StatsRequest
		response := &verifyapi.StatsResponse{}
		code, err := jsonutil.Unmarshal(w, r, &request)
		if err != nil {
			message := fmt.Sprintf("error unmarshalling API call, code: %v: %v", code, err)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			errorCode := verifyapi.ErrorBadRequest
			if code == http.StatusInternalServerError {
				errorCode = verifyapi.ErrorInternalError
			}
			s.addMetricsPadding(ctx, response)
			jsonutil.MarshalResponse(w, http.StatusBadRequest, &verifyapi.StatsResponse{
				ErrorMessage: message,
				ErrorCode:    errorCode,
			})
			return
		}

		response, status := s.handleMetricsRequest(ctx, r.Header.Get("Authorization"), &request)
		s.addMetricsPadding(ctx, response)

		jsonutil.MarshalResponse(w, status, response)
	})
}

func (s *Server) addMetricsPadding(ctx context.Context, response *verifyapi.StatsResponse) {
	logger := logging.FromContext(ctx).Named("addMetricsPadding")

	if padding, err := generatePadding(s.config.StatsResponsePaddingMinBytes, s.config.StatsResponsePaddingRange); err != nil {
		logger.Errorw("failed to pad response", "error", err)
	} else {
		response.Padding = padding
	}
}

func (s *Server) handleMetricsRequest(ctx context.Context, bearerToken string, request *verifyapi.StatsRequest) (*verifyapi.StatsResponse, int) {
	logger := logging.FromContext(ctx).Named("handleMetricsRequest")

	response := &verifyapi.StatsResponse{}

	if !strings.HasPrefix(bearerToken, "Bearer ") {
		response.ErrorMessage = "Authorization header is not in `Bearer <token>` format"
		response.ErrorCode = verifyapi.ErrorUnauthorized
		return response, http.StatusUnauthorized
	}
	// Remove 'Bearer ' from the token.
	bearerToken = bearerToken[7:]

	// Validate JWT - if valid, the health authority ID (based on issuer) is returned.
	healthAuthorityID, err := s.verifier.AuthenticateStatsToken(ctx, bearerToken)
	if err != nil {
		logger.Infow("stats authorization failure", "error", err)
		response.ErrorMessage = err.Error()
		response.ErrorCode = verifyapi.ErrorUnauthorized
		return response, http.StatusUnauthorized
	}

	// retrieve stats
	stats, err := s.database.ReadStats(ctx, healthAuthorityID)
	if err != nil {
		logger.Errorw("error reading stats", "error", err)
		response.ErrorMessage = "error reading stats"
		response.ErrorCode = verifyapi.ErrorInternalError
		return response, http.StatusInternalServerError
	}

	// Nothing from the current hour can be shown.
	onlyBefore := time.Now().UTC().Truncate(time.Hour)

	// Combine days - this also filters things that are "too new" and days that don't meet the threshold.
	response.Days = model.ReduceStats(stats, onlyBefore, s.config.StatsUploadMinimum)

	// return
	return response, http.StatusOK
}
