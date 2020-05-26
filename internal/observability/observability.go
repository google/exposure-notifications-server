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

package observability

import (
	"os"
	"sync"

	"contrib.go.opencensus.io/exporter/ocagent"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

func init() {
	initExporterOnce.Do(initExporter)
}

var initExporterOnce sync.Once

func initExporter() {
	exp := exporter()
	if exp == nil {
		return
	}
	trace.ApplyConfig(trace.Config{
		// Sample 40% of the traces.
		// TODO: Get the default sampling rate from a configuration
		// or just use OpenCensus' default of 1 in 10,000.
		DefaultSampler: trace.ProbabilitySampler(0.40),
	})
	trace.RegisterExporter(exp)
	view.RegisterExporter(exp)

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

type traceAndViewExporter interface {
	trace.Exporter
	view.Exporter
}

func exporter() traceAndViewExporter {
	switch os.Getenv("OBSERVABILITY_EXPORTER") {
	default:
		// TODO: Add other trace and view exporters and break out of this.
		return nil

	case "ocagent":
		// In here we'll initialize the Stackdriver exporter.
		oce, err := ocagent.NewExporter(ocagent.WithInsecure(), ocagent.WithAddress("localhost:55678"))
		if err != nil {
			panic(err)
		}
		return oce

	case "stackdriver":
		sde, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID: os.Getenv("PROJECT_ID"),
		})
		if err != nil {
			panic(err)
		}
		return sde
	}
}
