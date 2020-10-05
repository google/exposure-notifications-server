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
	"sync"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

var collectedViews = struct {
	views []*view.View
	sync.Mutex
}{
	views: append(append(append(append([]*view.View{}, ochttp.DefaultClientViews...), ochttp.DefaultServerViews...), ocgrpc.DefaultClientViews...), ocgrpc.DefaultServerViews...),
}

// CollectViews collects all the OpenCensus views and register at a later time
// when we setup the metric exporter.
// This is mainly to be able to "register" the views in a module's init(), but
// still be able to handle the errors correctly.
// Typical usage:
// var v = view.View{...}
// func init() {
//   observability.ColectViews(v)
// }
// // Actual view registration happens in exporter.StartExporter().
func CollectViews(views ...*view.View) {
	collectedViews.Lock()
	defer collectedViews.Unlock()
	collectedViews.views = append(collectedViews.views, views...)
}

// AllViews returns the collected OpenCensus views.
func AllViews() []*view.View {
	collectedViews.Lock()
	defer collectedViews.Unlock()
	return collectedViews.views
}

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
