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

// Package server provides an opinionated http server.
package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"contrib.go.opencensus.io/exporter/prometheus"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/gorilla/mux"
)

type MetricsDoneFunc func() error

// ServeMetricsIfPrometheus serves the opencensus metrics at /metrics when
// OBSERVABILITY_EXPORTER set to "prometheus".
func ServeMetricsIfPrometheus(ctx context.Context) (MetricsDoneFunc, error) {
	logger := logging.FromContext(ctx)

	exporter := os.Getenv("OBSERVABILITY_EXPORTER")
	if strings.EqualFold(exporter, "prometheus") {
		metricsPort := os.Getenv("METRICS_PORT")
		if metricsPort == "" {
			return nil, fmt.Errorf("OBSERVABILITY_EXPORTER set to 'prometheus' but no METRICS_PORT set")
		}

		exporter, err := prometheus.NewExporter(prometheus.Options{})
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}

		r := mux.NewRouter()
		r.Handle("/metrics", exporter)
		srv := &http.Server{
			Addr:              ":" + metricsPort,
			ReadHeaderTimeout: 10 * time.Second,
			Handler:           r,
		}

		// Start the server in the background.
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Errorw("failed to serve prometheus metrics", "error", err)
				return
			}
		}()
		logger.Debugw("prometheus exporter is running", "port", metricsPort)

		// Create the shutdown closer.
		metricsDone := func() error {
			logger.Debugw("shutting down prometheus metrics exporter")

			shutdownCtx, done := context.WithTimeout(context.Background(), 10*time.Second)
			defer done()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("failed to shutdown prometheus metrics exporter: %w", err)
			}
			logger.Debugw("finished shutting down prometheus metrics exporter")

			return nil
		}

		return metricsDone, nil
	}

	return nil, nil
}
