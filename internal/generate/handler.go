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
	"go.uber.org/zap"

	"github.com/google/exposure-notifications-server/internal/pb"
	publishdb "github.com/google/exposure-notifications-server/internal/publish/database"
	publishmodel "github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/verification"
	verifyapi "github.com/google/exposure-notifications-server/pkg/api/v1"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/util"
)

func NewHandler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx).Named("generate")

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	transformer, err := publishmodel.NewTransformer(config)
	if err != nil {
		return nil, fmt.Errorf("model.NewTransformer: %w", err)
	}
	logger.Debugw("max keys per upload", "value", config.MaxKeysOnPublish)
	logger.Debugw("max interval start age", "value", config.MaxIntervalAge)
	logger.Debugw("truncate window", "value", config.TruncateWindow)

	return &generateHandler{
		serverenv:   env,
		transformer: transformer,
		config:      config,
		database:    publishdb.New(env.Database()),
		logger:      logger,
	}, nil
}

type generateHandler struct {
	config      *Config
	serverenv   *serverenv.ServerEnv
	transformer *publishmodel.Transformer
	database    *publishdb.PublishDB
	logger      *zap.SugaredLogger
}

func (h *generateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "(*generate.generateHandler).ServeHTTP")
	defer span.End()

	logger := h.logger.Named("ServeHTTP")

	regionStr := h.config.DefaultRegion
	if v := r.URL.Query().Get("region"); v != "" {
		regionStr = v
	}
	regions := strings.Split(regionStr, ",")

	logger.Debugw("generating data", "regions", regions)

	if err := h.generate(ctx, regions); err != nil {
		logger.Errorw("failed to generate", "error", err)
		span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: err.Error()})
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "successfully generated exposure keys")
}

func (h *generateHandler) generate(ctx context.Context, regions []string) error {
	logger := h.logger.Named("generate")

	// We require at least 2 keys because revision only revises a subset of keys,
	// and that subset selects a random sample from (0-len(keys)], and rand panics
	// if you try to generate a random number between 0 and 0 :).
	if h.config.KeysPerExposure < 2 {
		return fmt.Errorf("number of keys to publish must be at least 2")
	}

	batchTime := time.Now().UTC()
	for i := 0; i < h.config.NumExposures; i++ {
		logger.Debugf("generating exposure %d of %d", i+1, h.config.NumExposures)

		publish := &verifyapi.Publish{
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
				return fmt.Errorf("failed to simulate same day key release: %w", err)
			}

			lastKey.IntervalCount = lastKey.IntervalCount / 2
			publish.Keys = append(publish.Keys, newLastDayKey)
		}

		val, err := util.RandomInt(100)
		if err != nil {
			return fmt.Errorf("failed to decide revised key status: %w", err)
		}
		generateRevisedKeys := val <= h.config.ChanceOfKeyRevision

		reportType := verifyapi.ReportTypeClinical
		if !generateRevisedKeys {
			reportType, err = util.RandomReportType()
			if err != nil {
				return fmt.Errorf("failed to generate report type: %w", err)
			}
		}

		intervalIdx, err := util.RandomInt(len(publish.Keys) - 1)
		if err != nil {
			return fmt.Errorf("failed to generate symptom onset interval: %w", err)
		}

		claims := verification.VerifiedClaims{
			ReportType:           reportType,
			SymptomOnsetInterval: uint32(publish.Keys[intervalIdx].IntervalNumber),
		}

		exposures, err := h.transformer.TransformPublish(ctx, publish, regions, &claims, batchTime)
		if err != nil {
			return fmt.Errorf("failed to transform generated exposures: %w", err)
		}

		n, err := h.database.InsertAndReviseExposures(ctx, exposures, nil, true)
		if err != nil {
			return fmt.Errorf("failed to write exposure record: %w", err)
		}
		logger.Debugw("generated exposures", "num", n)

		if generateRevisedKeys {
			revisedReportType, err := util.RandomRevisedReportType()
			if err != nil {
				return fmt.Errorf("failed to generate revised report type: %w", err)
			}

			claims.ReportType = revisedReportType
			batchTime = batchTime.Add(h.config.KeyRevisionDelay)

			exposures, err := h.transformer.TransformPublish(ctx, publish, regions, &claims, batchTime)
			if err != nil {
				return fmt.Errorf("failed to transform generated exposures: %w", err)
			}

			// Build the revision token
			var token pb.RevisionTokenData
			for _, e := range exposures {
				token.RevisableKeys = append(token.RevisableKeys, &pb.RevisableKey{
					TemporaryExposureKey: e.ExposureKey,
					IntervalNumber:       e.IntervalNumber,
					IntervalCount:        e.IntervalCount,
				})
			}

			n, err := h.database.InsertAndReviseExposures(ctx, exposures, &token, true)
			if err != nil {
				return fmt.Errorf("failed to revise exposure record: %w", err)
			}
			logger.Debugw("revised exposures", "num", n)
		}
	}

	return nil
}
