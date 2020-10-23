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
	"github.com/google/exposure-notifications-server/pkg/api/v1alpha1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/mikehelmick/go-chaff"
)

func (h *PublishHandler) handleV1Apha1Request(w http.ResponseWriter, r *http.Request) *response {
	ctx, span := trace.StartSpan(r.Context(), "(*publish.PublishHandler).handleRequest")
	defer span.End()

	metrics := h.serverenv.MetricsExporter(ctx)
	metricsMiddleware := metricsware.NewMiddleWare(&metrics)

	w.Header().Set(HeaderAPIVersion, "v1alpha")

	var data v1alpha1.Publish
	code, err := jsonutil.Unmarshal(w, r, &data)
	if err != nil {
		message := fmt.Sprintf("error unmarshalling API call, code: %v: %v", code, err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
		return &response{
			status:      code,
			pubResponse: &verifyapi.PublishResponse{ErrorMessage: message}, // will be down-converted in ServeHTTP
			metrics: func() {
				metricsMiddleware.RecordBadJSON(ctx)
			},
		}
	}

	// Upconvert the exposure key records.
	v1keys := make([]verifyapi.ExposureKey, len(data.Keys))
	for i, k := range data.Keys {
		v1keys[i] = verifyapi.ExposureKey{
			Key:              k.Key,
			IntervalNumber:   k.IntervalNumber,
			IntervalCount:    k.IntervalCount,
			TransmissionRisk: k.TransmissionRisk,
		}
	}

	// Upconvert v1alpha1 to verifyapi.
	publish := verifyapi.Publish{
		Keys:                 v1keys,
		HealthAuthorityID:    data.AppPackageName,
		VerificationPayload:  data.VerificationPayload,
		HMACKey:              data.HMACKey,
		SymptomOnsetInterval: data.SymptomOnsetInterval,
		Traveler:             len(data.Regions) > 1, // if the key is in more than 1 region, upgrade to traveler status.
		RevisionToken:        data.RevisionToken,
		Padding:              data.Padding,
	}
	bridge := newVersionBridge(data.Regions)

	return h.process(ctx, &publish, bridge)
}

func (h *PublishHandler) HandleV1Alpha1() http.Handler {
	return h.tracker.HandleTrack(chaff.HeaderDetector("X-Chaff"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := h.handleV1Apha1Request(w, r)

			ctx := r.Context()
			metrics := h.serverenv.MetricsExporter(ctx)
			metricsMiddleware := metricsware.NewMiddleWare(&metrics)

			if err := response.padResponse(h.config); err != nil {
				metricsMiddleware.RecordPaddingFailure(ctx)
				logging.FromContext(ctx).Errorw("failed to padd response", "error", err)
			}

			// Downgrade the v1 response to a v1alpha1 response.
			alpha1Response := v1alpha1.PublishResponse{
				RevisionToken:     response.pubResponse.RevisionToken,
				InsertedExposures: response.pubResponse.InsertedExposures,
				Error:             response.pubResponse.ErrorMessage,
				Padding:           response.pubResponse.Padding,
				Warnings:          response.pubResponse.Warnings,
			}

			if response.metrics != nil {
				response.metrics()
			}

			data, err := json.Marshal(&alpha1Response)
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
