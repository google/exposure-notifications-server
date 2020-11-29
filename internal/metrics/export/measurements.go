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

// Package export contains OpenCensus metrics and views for export operations
package export

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	exportMetricsPrefix = metrics.MetricRoot + "export"

	BatcherLockContention = stats.Int64(exportMetricsPrefix+"export_batcher_lock_contention",
		"Instances of export batcher lock contention", stats.UnitDimensionless)
	BatcherFailure = stats.Int64(exportMetricsPrefix+"export_batcher_failure",
		"Instances of export batcher failures", stats.UnitDimensionless)
	BatcherNoWork = stats.Int64(exportMetricsPrefix+"export_batcher_no_work",
		"Instances of export batcher having no work", stats.UnitDimensionless)
	BatcherCreated = stats.Int64(exportMetricsPrefix+"export_batches_created",
		"Number of export batchers created", stats.UnitDimensionless)
	WorkerBadKeyLength = stats.Int64(exportMetricsPrefix+"export_worker_bad_key_length",
		"Number of dropped keys caused by bad key length", stats.UnitDimensionless)
)
