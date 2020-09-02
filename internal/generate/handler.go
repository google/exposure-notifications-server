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

package generate

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/util"
)

func NewHandler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx)

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	transformer, err := model.NewTransformer(config)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}
	logger.Infof("max keys per upload: %v", config.MaxKeysOnPublish)
	logger.Infof("max interval start age: %v", config.MaxIntervalAge)
	logger.Infof("truncate window: %v", config.TruncateWindow)

	return &generateHandler{
		serverenv:   env,
		transformer: transformer,
		config:      config,
		database:    database.New(env.Database()),
	}, nil
}

type generateHandler struct {
	config      *Config
	serverenv   *serverenv.ServerEnv
	transformer *model.Transformer
	database    *database.PublishDB
}

func (h *generateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*generate.generateHandler).ServeHTTP")
	defer span.End()
	logger := logging.FromContext(ctx)

	regionStr := h.config.DefaultRegion
	if regionParams, ok := r.URL.Query()["region"]; ok && len(regionParams) > 0 {
		regionStr = regionParams[0]
	}
	regions := strings.Split(regionStr, ",")
	logger.Infof("Request to generate data for regions: %v", regions)

	batchTime := time.Now().UTC()
	for i := 0; i < h.config.NumExposures; i++ {
		logger.Infof("Generating exposure %v of %v", i+1, h.config.NumExposures)

		publish := verifyapi.Publish{
			Keys:              util.GenerateExposureKeys(h.config.KeysPerExposure, 0, false),
			HealthAuthorityID: "generated.data",
		}
		if h.config.SimulateSameDayRelease {
			sort.Slice(publish.Keys, func(i int, j int) bool {
				return publish.Keys[i].IntervalNumber < publish.Keys[j].IntervalNumber
			})
			lastKey := &publish.Keys[len(publish.Keys)-1]
			newLastDayKey, err := util.RandomExposureKey(lastKey.IntervalNumber, 144, lastKey.TransmissionRisk)
			if err != nil {
				logger.Errorf("unable to simulate same day key release: %v", err)
			} else {
				lastKey.IntervalCount = lastKey.IntervalCount / 2
				publish.Keys = append(publish.Keys, newLastDayKey)
			}
		}

		val, err := util.RandomInt(100)
		if err != nil {
			message := fmt.Sprintf("error deciding on revised key status: %v", err)
			logger.Errorf(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, message)
			return
		}
		generateRevisedKeys := val <= h.config.ChanceOfKeyRevision

		reportType := verifyapi.ReportTypeClinical
		if !generateRevisedKeys {
			reportType, err = util.RandomReportType()
			if err != nil {
				message := fmt.Sprintf("error generating report type: %v", err)
				logger.Errorf(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}
		}

		intervalIdx, err := util.RandomInt(len(publish.Keys) - 1)
		if err != nil {
			message := fmt.Sprintf("error generating symptom onset interval: %v", err)
			logger.Errorf(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, message)
			return
		}

		claims := verification.VerifiedClaims{
			ReportType:           reportType,
			SymptomOnsetInterval: uint32(publish.Keys[intervalIdx].IntervalNumber),
		}

		exposures, err := h.transformer.TransformPublish(ctx, &publish, regions, &claims, batchTime)
		if err != nil {
			message := fmt.Sprintf("Error transforming generated exposures: %v", err)
			logger.Errorf(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, message)
			return
		}

		n, err := h.database.InsertAndReviseExposures(ctx, exposures, nil, false, false)
		if err != nil {
			message := fmt.Sprintf("error writing exposure record: %v", err)
			logger.Errorf(message)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, message)
			return
		}
		logger.Infof("Generated %v exposures", n)

		if generateRevisedKeys {
			revisedReportType, err := util.RandomRevisedReportType()
			if err != nil {
				message := fmt.Sprintf("error generating revised report type: %v", err)
				logger.Errorf(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}

			claims.ReportType = revisedReportType
			batchTime = batchTime.Add(h.config.KeyRevisionDelay)

			exposures, err := h.transformer.TransformPublish(ctx, &publish, regions, &claims, batchTime)
			if err != nil {
				message := fmt.Sprintf("Error transforming generated exposures: %v", err)
				logger.Errorf(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}

			// Bypass revision token enforcement on generated data.
			n, err := h.database.InsertAndReviseExposures(ctx, exposures, nil, false, false)
			if err != nil {
				message := fmt.Sprintf("error writing exposure record: %v", err)
				logger.Errorf(message)
				span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, message)
				return
			}
			logger.Infof("Revised %v exposures", n)
		}
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Generated exposure keys.")
}
