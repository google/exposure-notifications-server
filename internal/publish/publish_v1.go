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
	"fmt"
	"net/http"

	"go.opencensus.io/stats"
	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/maintenance"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/mikehelmick/go-chaff"
)

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) *response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.PublishHandler).handleRequest")
	defer span.End()

	w.Header().Set(HeaderAPIVersion, "v1")

	var data verifyapi.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
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
			metrics: func() {
				stats.Record(ctx, mBadJSON.M(1))
			},
		}
	}

	clientPlatform := platform(r.UserAgent())
	return s.process(ctx, &data, clientPlatform, newVersionBridge([]string{}))
}

// Handle returns an http.Handler that can process V1 publish requests.
func (s *Server) Handle() http.Handler {
	mResponder := maintenance.New(s.config)
	return s.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		mResponder.Handle(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				response := s.handleRequest(w, r)

				ctx := r.Context()

				if padding, err := generatePadding(s.config.ResponsePaddingMinBytes, s.config.ResponsePaddingRange); err != nil {
					stats.Record(ctx, mPaddingFailed.M(1))
					logging.FromContext(ctx).Errorw("failed to pad response", "error", err)
				} else {
					response.pubResponse.Padding = padding
				}

				if response.metrics != nil {
					response.metrics()
				}

				jsonutil.MarshalResponse(w, response.status, response.pubResponse)
			})))
}
