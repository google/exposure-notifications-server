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
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	exportMetricsPrefix = metrics.MetricRoot + "export"

	mBatcherLockContention = stats.Int64(exportMetricsPrefix+"export_batcher_lock_contention",
		"Instances of export batcher lock contention", stats.UnitDimensionless)
	mBatcherFailure = stats.Int64(exportMetricsPrefix+"export_batcher_failure",
		"Instances of export batcher failures", stats.UnitDimensionless)
	mBatcherNoWork = stats.Int64(exportMetricsPrefix+"export_batcher_no_work",
		"Instances of export batcher having no work", stats.UnitDimensionless)
	mBatcherCreated = stats.Int64(exportMetricsPrefix+"export_batches_created",
		"Number of export batchers created", stats.UnitDimensionless)
	mWorkerBadKeyLength = stats.Int64(exportMetricsPrefix+"export_worker_bad_key_length",
		"Number of dropped keys caused by bad key length", stats.UnitDimensionless)

	mExportBatchCompletion = stats.Int64(exportMetricsPrefix+"export_batch_completion",
		"Number of batches complete by output region", stats.UnitDimensionless)

	ExportConfigIDTagKey  = tag.MustNewKey("export_config_id")
	ExportRegionTagKey    = tag.MustNewKey("export_region")
	ExportTravelersTagKey = tag.MustNewKey("includes_travelers")
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "export_batcher_lock_contention_count",
			Description: "Total count of lock contention instances",
			Measure:     mBatcherLockContention,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_failure_count",
			Description: "Total count of export batcher failures",
			Measure:     mBatcherFailure,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_no_work_count",
			Description: "Total count for instances of export batcher having no work",
			Measure:     mBatcherNoWork,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batches_created_count",
			Description: "Total count for number of export batches created",
			Measure:     mBatcherCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_worker_bad_key_length_latest",
			Description: "Latest number of dropped keys caused by bad key length",
			Measure:     mWorkerBadKeyLength,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "export_batch_completion",
			Description: "Number of batches complete by output region",
			Measure:     mExportBatchCompletion,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{ExportConfigIDTagKey, ExportRegionTagKey, ExportTravelersTagKey},
		},
	}...)
}
