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
)

// Exporter defines the minimum shared functionality for an observability exporter
// used by this application.
type Exporter interface {
	io.Closer
	StartExporter(func() error) error
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
