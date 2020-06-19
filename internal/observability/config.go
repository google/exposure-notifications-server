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

	OpenCensusConfig
	StackdriverConfig
	OCAgentConfig
}

// OpenCensusConfig holds the configuration options for the open census exporter
type OpenCensusConfig struct {
	TraceProbabilitySampleRate float64 `env:"TRACE_PROBABILITY, default=0.40"`
}

// StackdriverConfig holds the configuration options for the stackdriver exporter
type StackdriverConfig struct {
	ProjectID string `env:"PROJECT_ID, default=$GOOGLE_CLOUD_PROJECT"`
}

// OCAgentConfig holds the configuration options for the default opencensus exporter
type OCAgentConfig struct {
	Insecure bool   `env:"OCAGENT_INSECURE, default=true"`
	Endpoint string `env:"OCAGENT_TRACE_EXPORTER_ENDPOINT"`
}
