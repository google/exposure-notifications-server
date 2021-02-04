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
	"database/sql"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/exposure-notifications-server/internal/buildinfo"

	"contrib.go.opencensus.io/integrations/ocsql"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// OCSQLDriverName is the name of the SQL driver wrapped by OpenCensus
// instrumentation code.
const OCSQLDriverName = "ocsql"

var (
	BuildIDTagKey  = tag.MustNewKey("build_id")
	BuildTagTagKey = tag.MustNewKey("build_tag")

	KnativeServiceTagKey       = tag.MustNewKey("k_service")
	KnativeRevisionTagKey      = tag.MustNewKey("k_revision")
	KnativeConfigurationTagKey = tag.MustNewKey("k_configuration")

	knativeService       = os.Getenv("K_SERVICE")
	knativeRevision      = os.Getenv("K_REVISION")
	knativeConfiguration = os.Getenv("K_CONFIGURATION")
)

func defaultViews() []*view.View {
	var ret []*view.View
	ret = append(ret, ochttp.DefaultClientViews...)
	ret = append(ret, ochttp.DefaultServerViews...)
	ret = append(ret, ocgrpc.DefaultClientViews...)
	ret = append(ret, ocgrpc.DefaultServerViews...)

	for _, d := range sql.Drivers() {
		if d == OCSQLDriverName {
			ret = append(ret, ocsql.DefaultViews...)
			break
		}
	}
	return ret
}

var collectedViews = struct {
	views []*view.View
	sync.Mutex
}{}

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
	return append(collectedViews.views, defaultViews()...)
}

// Exporter defines the minimum shared functionality for an observability exporter
// used by this application.
type Exporter interface {
	io.Closer
	StartExporter(ctx context.Context) error
}

// NewFromEnv returns the observability exporter given the provided configuration, or an error
// if it failed to be created.
func NewFromEnv(config *Config) (Exporter, error) {
	// Create a separate ctx.
	// The main ctx will be canceled when the server is shutting down. Sharing
	// the main ctx prevent the last batch of the metrics to be uploaded.
	ctx := context.Background()
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

// WithBuildInfo creates a new context with the build and revision info attached
// to the observability context.
func WithBuildInfo(ctx context.Context) context.Context {
	tags := make([]tag.Mutator, 0, 5)
	tags = append(tags, tag.Upsert(BuildIDTagKey, buildinfo.BuildID))
	tags = append(tags, tag.Upsert(BuildTagTagKey, buildinfo.BuildTag))

	if knativeService != "" {
		tags = append(tags, tag.Upsert(KnativeServiceTagKey, knativeService))
	}

	if knativeRevision != "" {
		tags = append(tags, tag.Upsert(KnativeRevisionTagKey, knativeRevision))
	}

	if knativeConfiguration != "" {
		tags = append(tags, tag.Upsert(KnativeConfigurationTagKey, knativeConfiguration))
	}

	ctx, _ = tag.New(ctx, tags...)
	return ctx
}
