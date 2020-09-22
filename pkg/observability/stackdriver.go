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

// Package observability sets up and configures observability tools.
package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.uber.org/zap"
	"google.golang.org/api/option"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var _ Exporter = (*stackdriverExporter)(nil)

type stackdriverExporter struct {
	exporter *stackdriver.Exporter
	config   *StackdriverConfig
}

type debugRoundTripper struct {
	logger *zap.SugaredLogger
	http.RoundTripper
}

func (r *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.RoundTripper.RoundTrip(req)
	r.logger.Debugw("http debug", "request", req, "response", resp, "err", err)
	return resp, err
}

// NewStackdriver creates a new metrics and trace exporter for Stackdriver.
func NewStackdriver(ctx context.Context, config *StackdriverConfig) (Exporter, error) {
	logger := logging.FromContext(ctx).Named("stackdriver")

	projectID := config.ProjectID
	if projectID == "" {
		return nil, fmt.Errorf("missing PROJECT_ID in Stackdriver exporter")
	}

	monitoredResource := NewStackdriverMonitoredResource(ctx, config)
	logger.Debugw("monitored resource", "resource", monitoredResource)

	var opts []option.ClientOption

	if config.DebugLogging {
		c := &http.Client{
			Transport: &debugRoundTripper{
				RoundTripper: http.DefaultTransport,
				logger:       logging.FromContext(ctx).Named("debugRoundTripper"),
			},
		}
		opts = append(opts, option.WithHTTPClient(c))
	}

	exporter, err := stackdriver.NewExporter(stackdriver.Options{
		Context:                 ctx,
		ProjectID:               projectID,
		ReportingInterval:       config.ReportingInterval,
		BundleDelayThreshold:    config.BundleDelayThreshold,
		Timeout:                 config.Timeout,
		BundleCountThreshold:    int(config.BundleCountThreshold),
		MonitoredResource:       monitoredResource,
		DefaultMonitoringLabels: &stackdriver.Labels{},
		MonitoringClientOptions: opts,
		OnError: func(err error) {
			logger.Errorw("failed to export metric", "error", err, "resource", monitoredResource)
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create Stackdriver exporter: %w", err)
	}
	return &stackdriverExporter{exporter, config}, nil
}

// StartExporter starts the exporter.
func (e *stackdriverExporter) StartExporter() error {
	if err := registerViews(); err != nil {
		return fmt.Errorf("failed to register views: %w", err)
	}

	if err := e.exporter.StartMetricsExporter(); err != nil {
		return fmt.Errorf("failed to start stackdriver exporter: %w", err)
	}

	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.ProbabilitySampler(e.config.SampleRate),
	})
	trace.RegisterExporter(e.exporter)

	view.RegisterExporter(e.exporter)

	return nil
}

// Close halts the exporter.
func (e *stackdriverExporter) Close() error {
	// Flush any existing metrics
	e.exporter.Flush()

	// Stop the exporter
	e.exporter.StopMetricsExporter()

	// Unregister the exporter
	trace.UnregisterExporter(e.exporter)
	view.UnregisterExporter(e.exporter)

	return nil
}
