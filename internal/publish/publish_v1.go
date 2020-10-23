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
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/metrics/metricsware"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/mikehelmick/go-chaff"
)

func (h *PublishHandler) handleRequest(w http.ResponseWriter, r *http.Request) *response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.PublishHandler).handleRequest")
	defer span.End()

	metrics := h.serverenv.MetricsExporter(ctx)
	metricsMiddleware := metricsware.NewMiddleWare(&metrics)

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
				metricsMiddleware.RecordBadJSON(ctx)
			},
		}
	}

	return h.process(ctx, &data, newVersionBridge([]string{}))
}

// Handle returns an http.Handler that can process V1 publish requests.
func (h *PublishHandler) Handle() http.Handler {
	return h.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := h.handleRequest(w, r)

			ctx := r.Context()
			metrics := h.serverenv.MetricsExporter(ctx)
			metricsMiddleware := metricsware.NewMiddleWare(&metrics)

			if err := response.padResponse(h.config); err != nil {
				metricsMiddleware.RecordPaddingFailure(ctx)
				logging.FromContext(ctx).Errorw("failed to pad response", "error", err)
			}

			if response.metrics != nil {
				response.metrics()
			}

			data, err := json.Marshal(response.pubResponse)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "{\"error\": \"%v\"}", err.Error())
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(response.status)
			fmt.Fprintf(w, "%s", data)
		}))
}
