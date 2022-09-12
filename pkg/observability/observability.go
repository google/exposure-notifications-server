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
	"database/sql"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// OCSQLDriverName is the name of the SQL driver wrapped by OpenCensus
// instrumentation code.
const OCSQLDriverName = "ocsql"

var (
	// BlameTagKey indicating Who to blame for the API request failure.
	// NONE: no failure
	// CLIENT: the client is at fault (e.g. invalid request)
	// SERVER: the server is at fault
	// EXTERNAL: some third party is at fault
	// UNKNOWN: for everything else.
	BlameTagKey = tag.MustNewKey("blame")

	// ResultTagKey contains a free format text describing the result of the
	// request. Preferably ALL CAPS WITH UNDERSCORE.
	// OK indicating a successful request.
	// You can losely base this string on
	// https://github.com/googleapis/googleapis/blob/master/google/rpc/code.proto
	// but feel free to use any text as long as it's easy to filter.
	ResultTagKey = tag.MustNewKey("result")
)

var (
	// BlameNone indicate no API failure.
	BlameNone = tag.Upsert(BlameTagKey, "NONE")

	// BlameClient indicate the client is at fault (e.g. invalid request).
	BlameClient = tag.Upsert(BlameTagKey, "CLIENT")

	// BlameServer indicate the server is at fault.
	BlameServer = tag.Upsert(BlameTagKey, "SERVER")

	// BlameExternal indicate some third party is at fault.
	BlameExternal = tag.Upsert(BlameTagKey, "EXTERNAL")

	// BlameUnknown can be used for everything else.
	BlameUnknown = tag.Upsert(BlameTagKey, "UNKNOWN")
)

var (
	// ResultOK add a tag indicating the API call is a success.
	ResultOK = tag.Upsert(ResultTagKey, "OK")
	// ResultNotOK add a tag indicating the API call is a failure.
	ResultNotOK = ResultError("NOT_OK")
)

// ResultError add a tag with the given string as the result.
func ResultError(result string) tag.Mutator {
	return tag.Upsert(ResultTagKey, result)
}

// BuildInfo is the interface to provide build information.
type BuildInfo interface {
	ID() string
	Tag() string
}

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
//
//	var v = view.View{...}
//
//	func init() {
//	  observability.ColectViews(v)
//	}
//
// Actual view registration happens in [exporter.StartExporter].
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

// WithBuildInfo creates a new context with the build and revision info attached
// to the observability context.
func WithBuildInfo(ctx context.Context, info BuildInfo) context.Context {
	tags := make([]tag.Mutator, 0, 5)
	tags = append(tags, tag.Upsert(BuildIDTagKey, info.ID()))
	tags = append(tags, tag.Upsert(BuildTagTagKey, info.Tag()))

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

// RecordLatency calculate and record the latency:
//
//	func foo() {
//	  defer RecordLatency(&ctx, time.Now(), metric, tag1, tag2)
//	  // remaining of the function body.
//	}
func RecordLatency(ctx context.Context, start time.Time, m *stats.Float64Measure, mutators ...*tag.Mutator) {
	additionalMutators := make([]tag.Mutator, 0, len(mutators))
	for _, t := range mutators {
		additionalMutators = append(additionalMutators, *t)
	}

	// Calculate the millisecond number as float64. time.Duration.Millisecond()
	// returns an integer.
	latency := float64(time.Since(start)) / float64(time.Millisecond)
	if err := stats.RecordWithTags(ctx, additionalMutators, m.M(latency)); err != nil {
		logging.FromContext(ctx).Named("observability").
			Errorw("failed to record latency", "error", err)
	}
}
