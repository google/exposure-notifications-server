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

import "time"

// ExporterType represents a type of observability exporter.
type ExporterType string

const (
	ExporterStackdriver ExporterType = "STACKDRIVER"
	ExporterPrometheus  ExporterType = "PROMETHEUS"
	ExporterOCAgent     ExporterType = "OCAGENT"
	ExporterNoop        ExporterType = "NOOP"
)

// Config holds all of the configuration options for the observability exporter
type Config struct {
	ExporterType ExporterType `env:"OBSERVABILITY_EXPORTER, default=STACKDRIVER"`

	OpenCensus  *OpenCensusConfig
	Stackdriver *StackdriverConfig
}

// OpenCensusConfig holds the configuration options for the open census exporter
type OpenCensusConfig struct {
	SampleRate float64 `env:"TRACE_PROBABILITY, default=0.40"`

	Insecure bool   `env:"OCAGENT_INSECURE"`
	Endpoint string `env:"OCAGENT_TRACE_EXPORTER_ENDPOINT"`
}

// StackdriverConfig holds the configuration options for the stackdriver exporter
type StackdriverConfig struct {
	SampleRate float64 `env:"TRACE_PROBABILITY, default=0.40"`

	ProjectID string `env:"PROJECT_ID, default=$GOOGLE_CLOUD_PROJECT"`

	// Knative+Cloud Run container contract envvars:
	//
	// https://cloud.google.com/run/docs/reference/container-contract#env-vars
	//
	// If present, can be used to configured the Stackdriver MonitoredResource
	// correctly.
	Service   string `env:"K_SERVICE"`
	Revision  string `env:"K_REVISION"`
	Namespace string `env:"K_CONFIGURATION, default=en"`

	// Allows for providing a real Google Cloud location when running locally for development.
	// This is ignored if a real location was found during discovery.
	LocationOverride string `env:"DEV_STACKDRIVER_LOCATION"`

	// The following options are mostly for tuning the metrics reporting
	// behavior. You normally should not change these values.
	// ReportingInterval: should be >=60s as stackdriver enforces 60s minimal
	// interval.
	// BundleDelayThreshold / BundleCountThreshold: the stackdriver exporter
	// uses https://google.golang.org/api/support/bundler, these two options
	// control the max delay/count for batching the data points into one
	// stackdriver request.
	ReportingInterval    time.Duration `env:"STACKDRIVER_REPORTING_INTERVAL, default=2m"`
	BundleDelayThreshold time.Duration `env:"STACKDRIVER_BUNDLE_DELAY_THRESHOLD, default=2s"`
	BundleCountThreshold int           `env:"STACKDRIVER_BUNDLE_COUNT_THRESHOLD, default=50"`
}
