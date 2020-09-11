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
	"io"

	"github.com/google/exposure-notifications-server/internal/metrics/cleanup"
	"github.com/google/exposure-notifications-server/internal/metrics/export"
	"github.com/google/exposure-notifications-server/internal/metrics/federationin"
	"github.com/google/exposure-notifications-server/internal/metrics/federationout"
	"github.com/google/exposure-notifications-server/internal/metrics/publish"
	"github.com/google/exposure-notifications-server/internal/metrics/rotate"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

// Exporter defines the minimum shared functionality for an observability exporter
// used by this application.
type Exporter interface {
	io.Closer
	StartExporter() error
}

// NewFromEnv returns the observability exporter given the provided configuration, or an error
// if it failed to be created.
func NewFromEnv(ctx context.Context, config *Config) (Exporter, error) {
	switch config.ExporterType {
	case ExporterNoop:
		return NewNoop(ctx)
	case ExporterStackdriver:
		return NewStackdriver(ctx, config.Stackdriver)
	case ExporterOCAgent, ExporterPrometheus:
		return NewOpenCensus(ctx, config.OpenCensus)
	default:
		return nil, fmt.Errorf("unknown observability exporter type %v", config.ExporterType)
	}
}

// registerViews registers the necessary tracing views.
func registerViews() error {
	// Record the various HTTP view to collect metrics.
	httpViews := append(ochttp.DefaultServerViews, ochttp.DefaultClientViews...)
	if err := view.Register(httpViews...); err != nil {
		return fmt.Errorf("failed to register http views: %w", err)
	}

	// Register the various gRPC views to collect metrics.
	gRPCViews := append(ocgrpc.DefaultServerViews, ocgrpc.DefaultClientViews...)
	if err := view.Register(gRPCViews...); err != nil {
		return fmt.Errorf("failed to register grpc views: %w", err)
	}

	if err := view.Register(cleanup.Views...); err != nil {
		return fmt.Errorf("failed to register cleanup metrics: %w", err)
	}

	if err := view.Register(export.Views...); err != nil {
		return fmt.Errorf("failed to register export metrics: %w", err)
	}

	if err := view.Register(federationin.Views...); err != nil {
		return fmt.Errorf("failed to register federationin metrics: %w", err)
	}

	if err := view.Register(federationout.Views...); err != nil {
		return fmt.Errorf("failed to register federationout metrics: %w", err)
	}

	if err := view.Register(publish.Views...); err != nil {
		return fmt.Errorf("failed to register publish metrics: %w", err)
	}

	if err := view.Register(rotate.Views...); err != nil {
		return fmt.Errorf("failed to register rotate metrics: %w", err)
	}

	return nil
}
