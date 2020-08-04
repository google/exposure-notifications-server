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
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/jsonutil"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	v1 "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/mikehelmick/go-chaff"
)

// NewV1Handler creates the HTTP handler for the TTK publishing API set at v1.
func NewV1Handler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	processor, err := newProcessor(ctx, config, env)
	if err != nil {
		return nil, err
	}

	return &v1Handler{processor: processor}, nil
}

type v1Handler struct {
	processor *publishHandler
}

func (h *v1Handler) handleRequest(w http.ResponseWriter, r *http.Request) response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.publishHandler).handleRequest")
	defer span.End()

	var data v1.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		message := fmt.Sprintf("error unmarshaling API call, code: %v: %v", code, err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status: http.StatusBadRequest,
			pubResponse: &v1.PublishResponse{
				ErrorMessage: message,
				ErrorCode:    v1.ErrorBadRequest,
			},
			metric: "publish-bad-json",
			count:  1,
		}
	}

	return h.processor.process(ctx, &data, newVersionBridge([]string{}))
}

// ServeHTTP handles the publish event. It tracks requests and can handle chaff
// requests when provided a request with the X-Chaff header.
func (h *v1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.processor.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := h.handleRequest(w, r)

			if response.metric != "" {
				ctx := r.Context()
				metrics := h.processor.serverenv.MetricsExporter(ctx)
				metrics.WriteInt(response.metric, true, response.count)
			}

			data, err := json.Marshal(response.pubResponse)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "{\"error\": \"%v\"}", err.Error())
				return
			}
			w.WriteHeader(response.status)
			fmt.Fprintf(w, "%s", data)
		})).ServeHTTP(w, r)
}
