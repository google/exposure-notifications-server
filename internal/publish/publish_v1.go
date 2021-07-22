// Copyright 2020 the Exposure Notifications Server authors
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
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	obs "github.com/google/exposure-notifications-server/pkg/observability"
)

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) *response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.PublishHandler).handleRequest")
	defer span.End()

	w.Header().Set(HeaderAPIVersion, "v1")

	var data verifyapi.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		if s.config.LogJSONParseErrors {
			logger := logging.FromContext(ctx).Named("handlePublishV1.handleRequest")
			logger.Warnw("v1 unmarshal failure", "error", err)
		}

		blame := obs.BlameClient
		obsResult := obs.ResultError("BAD_JSON")
		defer obs.RecordLatency(ctx, time.Now(), mLatencyMs, &blame, &obsResult)
		message := fmt.Sprintf("error unmarshalling API call, code: %v: %v", code, err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		errorCode := verifyapi.ErrorBadRequest
		if code == http.StatusInternalServerError {
			errorCode = verifyapi.ErrorInternalError
		}
		return &response{
			status: code,
			pubResponse: &verifyapi.PublishResponse{
				ErrorMessage: message,
				Code:         errorCode,
			},
		}
	}

	clientPlatform := platform(r.UserAgent())
	return s.process(ctx, &data, clientPlatform, newVersionBridge([]string{}))
}

// handlePublishV1 returns an http.Handler that can process V1 publish requests.
func (s *Server) handlePublishV1() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := logging.FromContext(ctx).Named("handlePublishV1")

		response := s.handleRequest(w, r)

		if padding, err := generatePadding(s.config.ResponsePaddingMinBytes, s.config.ResponsePaddingRange); err != nil {
			stats.Record(ctx, mPaddingFailed.M(1))
			logger.Errorw("failed to pad response", "error", err)
		} else {
			response.pubResponse.Padding = padding
		}

		jsonutil.MarshalResponse(w, response.status, response.pubResponse)
	})
}
