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
	"strings"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/exposure-notifications-server/internal/logging"
	"github.com/google/exposure-notifications-server/internal/publish/database"
	"github.com/google/exposure-notifications-server/internal/publish/model"
	"github.com/google/exposure-notifications-server/internal/serverenv"
	"github.com/google/exposure-notifications-server/internal/util"
)

func NewHandler(ctx context.Context, config *Config, env *serverenv.ServerEnv) (http.Handler, error) {
	logger := logging.FromContext(ctx)

	if env.Database() == nil {
		return nil, fmt.Errorf("missing database in server environment")
	}

	transformer, err := model.NewTransformer(config.MaxKeysOnPublish, config.MaxIntervalAge, config.TruncateWindow)
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
	logger.Infof("Rquest to generate data for regions: %v", regions)

	batchTime := time.Now()
	for i := 0; i < h.config.NumExposures; i++ {
		logger.Infof("Generating exposure %v of %v", i+1, h.config.NumExposures)

		tr, err := util.RandomTransmissionRisk()
		if err != nil {
			tr = 0
		}
		publish := model.Publish{
			Keys:           util.GenerateExposureKeys(h.config.KeysPerExposure, tr, false),
			Regions:        regions,
			AppPackageName: "generated.data",
		}

		exposures, err := h.transformer.TransformPublish(&publish, batchTime)
		if err != nil {
			message := fmt.Sprintf("Error transofmring generated exposures: %v", err)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(message))
			return
		}

		err = h.database.InsertExposures(ctx, exposures)
		if err != nil {
			message := fmt.Sprintf("error writing exposure record: %v", err)
			span.SetStatus(trace.Status{Code: trace.StatusCodeInternal, Message: message})
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(message))
			return
		}
		logger.Infof("Generated %v exposures", len(exposures))
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Generated exposure keys."))
}
