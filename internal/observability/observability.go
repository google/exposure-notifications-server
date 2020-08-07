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
	"time"

	"contrib.go.opencensus.io/exporter/ocagent"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"github.com/google/exposure-notifications-server/internal/logging"
)

// Exporter defines the minimum shared functionality for an observability exporter
// used by this application.
type Exporter interface {
	InitExportOnce() error
}

// NewFromEnv returns the observability exporter given the provided configuration, or an error
// if it failed to be created.
func NewFromEnv(ctx context.Context, config *Config) (Exporter, error) {
	switch config.ExporterType {
	default:
		return &NoopExporter{}, nil

	case ExporterNoop:
		return &NoopExporter{}, nil

	case ExporterStackdriver:
		if config.StackdriverConfig.ProjectID == "" {
			return nil, fmt.Errorf("configuration PROJECT_ID is required to use the Stackdriver observability exporter")
		}
		logger := logging.FromContext(ctx).Named("stackdriver")

		monitoredResource := NewStackdriverMonitoredResoruce(&config.StackdriverConfig)

		sde, err := stackdriver.NewExporter(stackdriver.Options{
			ProjectID:         config.StackdriverConfig.ProjectID,
			ReportingInterval: time.Minute, // stackdriver export interval minimum
			MonitoredResource: monitoredResource,
			OnError: func(err error) {
				logger.Errorf("stackdriver export error: %v", err)
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create Stackdriver observability exporter: %v", err)
		}
		return &GenericExporter{sde, config.TraceProbabilitySampleRate}, nil

	case ExporterOCAgent, ExporterPrometheus:
		var opts []ocagent.ExporterOption
		if config.OCAgentConfig.Insecure {
			opts = append(opts, ocagent.WithInsecure())
		}
		if config.OCAgentConfig.Endpoint != "" {
			opts = append(opts, ocagent.WithAddress(config.OCAgentConfig.Endpoint))
		}

		oce, err := ocagent.NewExporter(opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenCensus observability exporter: %v", err)
		}
		return &GenericExporter{oce, config.TraceProbabilitySampleRate}, nil
	}
}
