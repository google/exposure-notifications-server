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
	"sync"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

// Compile-time check to verify implements interface.
var _ Exporter = (*GenericExporter)(nil)

var initExporterOnce sync.Once

type traceAndViewExporter interface {
	trace.Exporter
	view.Exporter
}

// GenericExporter is a standard implementation of an exporter that wraps the opencensus interfaces
// with custom configuration
type GenericExporter struct {
	exporter   traceAndViewExporter
	sampleRate float64
}

func (g *GenericExporter) InitExportOnce() {
	initExporterOnce.Do(g.initExporter)
}

func (g *GenericExporter) initExporter() {
	if g.exporter == nil {
		return
	}
	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.ProbabilitySampler(g.sampleRate),
	})
	trace.RegisterExporter(g.exporter)
	view.RegisterExporter(g.exporter)

	// Record the various HTTP view to collect metrics.
	httpViews := append(ochttp.DefaultServerViews, ochttp.DefaultClientViews...)
	if err := view.Register(httpViews...); err != nil {
		panic(err)
	}
	// Register the various gRPC views to collect metrics.
	gRPCViews := append(ocgrpc.DefaultServerViews, ocgrpc.DefaultClientViews...)
	if err := view.Register(gRPCViews...); err != nil {
		panic(err)
	}
}
