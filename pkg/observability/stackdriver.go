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
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/google/exposure-notifications-server/pkg/logging"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"google.golang.org/api/option"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

var _ Exporter = (*stackdriverExporter)(nil)

type stackdriverExporter struct {
	exporter *stackdriver.Exporter
	config   *StackdriverConfig
	options  *stackdriver.Options
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

	// Always have at least 3 workers
	numWorkers := 3
	if v := runtime.NumCPU() - 1; v > numWorkers {
		numWorkers = v
	}

	options := stackdriver.Options{
		Context:                 ctx,
		ProjectID:               projectID,
		ReportingInterval:       config.ReportingInterval,
		BundleDelayThreshold:    config.BundleDelayThreshold,
		Timeout:                 config.Timeout,
		NumberOfWorkers:         numWorkers,
		BundleCountThreshold:    int(config.BundleCountThreshold),
		MonitoredResource:       monitoredResource,
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
	}, nil
}

func (e *stackdriverExporter) metricClient() (*monitoring.MetricClient, error) {
	opts := append(e.options.MonitoringClientOptions, option.WithUserAgent(e.options.UserAgent))
	ctx := e.options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return monitoring.NewMetricClient(ctx, opts...)
}

// StartExporter starts the exporter.
func (e *stackdriverExporter) StartExporter(ctx context.Context) error {
	logger := logging.FromContext(ctx).Named("stackdriver")
	mclient, err := e.metricClient()
	if err != nil {
		return fmt.Errorf("unable to create metric client: %w", err)
	}

	allViews := AllViews()
	descriptorCreateRequests := make([]*monitoringpb.CreateMetricDescriptorRequest, 0, len(allViews))

	for _, v := range allViews {
		shouldSkip := false
		for _, prefix := range e.config.ExcludedMetricPrefixes {
			if strings.HasPrefix(v.Name, prefix) {
				logger.Infof("skip registering view %q as it matches the prefix to exclude: %q", v.Name, prefix)
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
		md, err := e.exporter.ViewToMetricDescriptor(ctx, v)
		if err != nil {
			return fmt.Errorf("failed to convert view to MetricDescriptor: %w", err)
		}
		cmrdesc := &monitoringpb.CreateMetricDescriptorRequest{
			Name:             fmt.Sprintf("projects/%s", e.config.ProjectID),
			MetricDescriptor: md,
		}
		descriptorCreateRequests = append(descriptorCreateRequests, cmrdesc)
	}

	// register metrics in the background and don't block server startup on failures.
	go func() {
		// Create a separate timeout for metric registration.
		ctx, done := context.WithTimeout(context.Background(), time.Duration(5*len(descriptorCreateRequests))*time.Second)
		defer done()
		logger.Infow("starting metric registration")
		for _, cmrdesc := range descriptorCreateRequests {
			func() {
				subCtx, done := context.WithTimeout(ctx, 5*time.Second)
				defer done()
				_, err = mclient.CreateMetricDescriptor(subCtx, cmrdesc)
				if err != nil {
					logger.Errorw("failed to create MetricDescriptor", "metric", cmrdesc.Name, "error", err)
				}
			}()
		}
		logger.Infow("finished metric registration")
	}()

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
	// Flush any existing metrics
	e.exporter.Flush()

	// Stop the exporter
	e.exporter.StopMetricsExporter()

	// Unregister the exporter
	trace.UnregisterExporter(e.exporter)

	return nil
}
