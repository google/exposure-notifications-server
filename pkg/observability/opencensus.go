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

package observability

import (
	"context"
	"fmt"

	"contrib.go.opencensus.io/exporter/ocagent"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var _ Exporter = (*opencensusExporter)(nil)

type opencensusExporter struct {
	exporter *ocagent.Exporter
	config   *OpenCensusConfig
}

// NewOpenCensus creates a new metrics and trace exporter for OpenCensus.
func NewOpenCensus(ctx context.Context, config *OpenCensusConfig) (Exporter, error) {
	var opts []ocagent.ExporterOption
	if config.Insecure {
		opts = append(opts, ocagent.WithInsecure())
	}
	if config.Endpoint != "" {
		opts = append(opts, ocagent.WithAddress(config.Endpoint))
	}

	oc, err := ocagent.NewExporter(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create opencensus exporter: %w", err)
	}
	return &opencensusExporter{oc, config}, nil
}

// StartExporter starts the exporter.
func (e *opencensusExporter) StartExporter(_ context.Context) error {
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.ProbabilitySampler(e.config.SampleRate),
	})
	trace.RegisterExporter(e.exporter)
	view.RegisterExporter(e.exporter)

	for _, v := range AllViews() {
		if err := view.Register(v); err != nil {
			return fmt.Errorf("failed to start opencensus exporter: view registration failed: %w", err)
		}
	}

	return nil
}

// Close halts the exporter.
func (e *opencensusExporter) Close() error {
	if err := e.exporter.Stop(); err != nil {
		return fmt.Errorf("failed to stop exporter: %w", err)
	}

	// Unregister the exporter
	trace.UnregisterExporter(e.exporter)
	view.UnregisterExporter(e.exporter)

	return nil
}
