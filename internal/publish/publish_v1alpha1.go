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
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/mikehelmick/go-chaff"
)

// NewV1Alpha1Handler creates the HTTP handler for the TTK publishing API set at v1alpha1.
func NewV1Alpha1Handler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	processor, err := newProcessor(ctx, config, env)
	if err != nil {
		return nil, err
	}

	return &v1alpha1Handler{processor: processor}, nil
}

type v1alpha1Handler struct {
	processor *publishHandler
}

func (h *v1alpha1Handler) handleRequest(w http.ResponseWriter, r *http.Request) response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.publishHandler).handleRequest")
	defer span.End()

	var data v1alpha1.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		message := fmt.Sprintf("error unmarshaling API call, code: %v: %v", code, err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return response{
			status:      http.StatusBadRequest,
			pubResponse: &v1.PublishResponse{ErrorMessage: message}, // will be down-converted in ServeHTTP
			metric:      "publish-bad-json", count: 1}
	}

	// Upconvert the exposure key records.
	v1keys := make([]v1.ExposureKey, len(data.Keys))
	for i, k := range data.Keys {
		v1keys[i] = v1.ExposureKey{
			Key:              k.Key,
			IntervalNumber:   k.IntervalNumber,
			IntervalCount:    k.IntervalCount,
			TransmissionRisk: k.TransmissionRisk,
		}
	}

	// Upconvert v1alpha1 to v1.
	publish := v1.Publish{
		Keys:                 v1keys,
		HealthAuthorityID:    data.AppPackageName,
		VerificationPayload:  data.VerificationPayload,
		HMACKey:              data.HMACKey,
		SymptomOnsetInterval: data.SymptomOnsetInterval,
		Traveler:             data.Traveler,
		RevisionToken:        data.RevisionToken,
		Padding:              data.Padding,
	}
	bridge := newVersionBridge(data.Regions)

	return h.processor.process(ctx, &publish, bridge)
}

// ServeHTTP handles the publish event. It tracks requests and can handle chaff
// requests when provided a request with the X-Chaff header.
func (h *v1alpha1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.processor.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := h.handleRequest(w, r)

			// Downgrade the v1 response to a v1alpha1 response.
			alpha1Response := v1alpha1.PublishResponse{
				RevisionToken:     response.pubResponse.RevisionToken,
				InsertedExposures: response.pubResponse.InsertedExposures,
				Error:             response.pubResponse.ErrorMessage,
				Padding:           response.pubResponse.Padding,
			}

			if response.metric != "" {
				ctx := r.Context()
				metrics := h.processor.serverenv.MetricsExporter(ctx)
				metrics.WriteInt(response.metric, true, response.count)
			}

			data, err := json.Marshal(&alpha1Response)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "{\"error\": \"%v\"}", err.Error())
				return
			}
			w.WriteHeader(response.status)
			fmt.Fprintf(w, "%s", data)
		})).ServeHTTP(w, r)
}
