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

// Package observability sets up and configures observability tools.
package observability

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.uber.org/zap"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
)

var _ Exporter = (*stackdriverExporter)(nil)

type stackdriverExporter struct {
	exporter *stackdriver.Exporter
	config   *StackdriverConfig
	options  *stackdriver.Options
	logger   *zap.SugaredLogger
}

type StackdriverExporter interface {
	Exporter
	ViewToMetricDescriptor(v *view.View) (*metricpb.MetricDescriptor, error)
}

// NewStackdriver creates a new metrics and trace exporter for Stackdriver.
func NewStackdriver(ctx context.Context, config *StackdriverConfig) (StackdriverExporter, error) {
	logger := logging.FromContext(ctx).Named("stackdriver")

	projectID := config.ProjectID
	if projectID == "" {
		return nil, fmt.Errorf("missing PROJECT_ID in Stackdriver exporter")
	}

	monitoredResource := NewStackdriverMonitoredResource(ctx, config)
	logger.Debugw("monitored resource", "resource", monitoredResource)

	// Always have at least 3 workers
	numWorkers := 3
	if v := runtime.NumCPU() - 1; v > numWorkers {
		numWorkers = v
	}

	options := stackdriver.Options{
		ProjectID:               projectID,
		ReportingInterval:       config.ReportingInterval,
		BundleDelayThreshold:    config.BundleDelayThreshold,
		Timeout:                 config.Timeout,
		NumberOfWorkers:         numWorkers,
		BundleCountThreshold:    int(config.BundleCountThreshold),
		MonitoredResource:       monitoredResource,
		SkipCMD:                 !config.RegisterMetrics,
		DefaultMonitoringLabels: &stackdriver.Labels{},
		OnError: func(err error) {
			logger.Errorw("failed to export metric", "error", err, "resource", monitoredResource)
		},
	}
	exporter, err := stackdriver.NewExporter(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stackdriver exporter: %w", err)
	}
	return &stackdriverExporter{
		exporter: exporter,
		config:   config,
		options:  &options,
		logger:   logger,
	}, nil
}

// ViewToMetricDescriptor proxies to the internal ViewToMetricDescriptor
// implementation.
func (e *stackdriverExporter) ViewToMetricDescriptor(v *view.View) (*metricpb.MetricDescriptor, error) {
	return e.exporter.ViewToMetricDescriptor(context.Background(), v)
}

// StartExporter starts the exporter.
func (e *stackdriverExporter) StartExporter() error {
	e.logger.Debugw("starting observability exporter")
	defer e.logger.Debugw("finished starting observability exporter")

	allViews := AllViews()
	for _, v := range allViews {
		shouldSkip := false
		for _, prefix := range e.config.ExcludedMetricPrefixes {
			if strings.HasPrefix(v.Name, prefix) {
				e.logger.Infof("skip registering view %q as it matches the prefix to exclude: %q", v.Name, prefix)
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}
		if err := view.Register(v); err != nil {
			return fmt.Errorf("failed to start stackdriver exporter: view registration failed: %w", err)
		}
	}

	if err := e.exporter.StartMetricsExporter(); err != nil {
		return fmt.Errorf("failed to start stackdriver exporter: %w", err)
	}

	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.ProbabilitySampler(e.config.SampleRate),
	})
	trace.RegisterExporter(e.exporter)

	return nil
}

// Close halts the exporter.
func (e *stackdriverExporter) Close() error {
	e.logger.Debugw("closing observability exporter")
	defer e.logger.Debugw("finished closing observability exporter")

	// Flush any existing metrics
	e.exporter.Flush()

	// Stop the exporter
	e.exporter.StopMetricsExporter()

	// Unregister the exporter
	trace.UnregisterExporter(e.exporter)

	return nil
}
