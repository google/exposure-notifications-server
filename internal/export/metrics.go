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

const metricPrefix = metrics.MetricRoot + "export"

var (
	ExportConfigIDTagKey  = tag.MustNewKey("config_id")
	ExportRegionTagKey    = tag.MustNewKey("region")
	ExportTravelersTagKey = tag.MustNewKey("includes_travelers")
)

var (
	mBatcherSuccess = stats.Int64(metricPrefix+"/batcher/success", "successful batcher execution", stats.UnitDimensionless)
	mWorkerSuccess  = stats.Int64(metricPrefix+"/worker/success", "successful worker execution", stats.UnitDimensionless)

	mBatcherNoWork         = stats.Int64(metricPrefix+"/batcher_no_work", "Instances of export batcher having no work", stats.UnitDimensionless)
	mBatcherCreated        = stats.Int64(metricPrefix+"/batches_created", "Number of export batchers created", stats.UnitDimensionless)
	mWorkerBadKeyLength    = stats.Int64(metricPrefix+"/worker_bad_key_length", "Number of dropped keys caused by bad key length", stats.UnitDimensionless)
	mExportBatchCompletion = stats.Int64(metricPrefix+"/batch_completion", "Number of batches complete by output region", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/batcher/success",
			Description: "Number of batcher successes",
			Measure:     mBatcherSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/worker/success",
			Description: "Number of worker successes",
			Measure:     mWorkerSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metrics.MetricRoot + "/batcher_no_work_count",
			Description: "Total count for instances of export batcher having no work",
			Measure:     mBatcherNoWork,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "/batches_created_count",
			Description: "Total count for number of export batches created",
			Measure:     mBatcherCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "/worker_bad_key_length_latest",
			Description: "Latest number of dropped keys caused by bad key length",
			Measure:     mWorkerBadKeyLength,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "/batch_completion",
			Description: "Number of batches complete by output region",
			Measure:     mExportBatchCompletion,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{ExportConfigIDTagKey, ExportRegionTagKey, ExportTravelersTagKey},
		},
	}...)
}
