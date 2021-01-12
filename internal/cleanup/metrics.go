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

// Package cleanup contains OpenCensus metrics and views for cleanup operations
package cleanup

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	cleanupMetricsPrefix = metrics.MetricRoot + "cleanup"

	mExposuresSetupFailed = stats.Int64(cleanupMetricsPrefix+"exposures_setup_failed",
		"Instances of exposures setup failures", stats.UnitDimensionless)
	mExposuresCleanupBefore = stats.Int64(cleanupMetricsPrefix+"exposures_cleanup_before",
		"Exposures cleanup cutoff date", stats.UnitSeconds)
	mExposuresDeleteFailed = stats.Int64(cleanupMetricsPrefix+"exposures_delete_failed",
		"Instances of exposures delete failures", stats.UnitDimensionless)
	mExposuresDeleted = stats.Int64(cleanupMetricsPrefix+"exposures_deleted",
		"Exposures deletions", stats.UnitDimensionless)
	mExposuresStatsDeleteFailed = stats.Int64(cleanupMetricsPrefix+"exposures_stats_delete_failed",
		"Instances of exposures stats delete failures", stats.UnitDimensionless)
	mExposuresStatsDeleted = stats.Int64(cleanupMetricsPrefix+"exposures_stats_deleted",
		"Exposures stats deletions", stats.UnitDimensionless)
	mExportsSetupFailed = stats.Int64(cleanupMetricsPrefix+"exports_setup_failed",
		"Instances of export setup failures", stats.UnitDimensionless)
	mExportsCleanupBefore = stats.Int64(cleanupMetricsPrefix+"exports_cleanup_before",
		"Exports cleanup cutoff date", stats.UnitSeconds)
	mExportsDeleteFailed = stats.Int64(cleanupMetricsPrefix+"exports_delete_failed",
		"Instances of exports delete failures", stats.UnitDimensionless)
	mExportsDeleted = stats.Int64(cleanupMetricsPrefix+"exports_deleted",
		"Exports deletions", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "exposures_setup_failed_count",
			Description: "Total count of exposures setup failures",
			Measure:     mExposuresSetupFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_cleanup_before_latest",
			Description: "Last value of exposures cleanup cutoff date",
			Measure:     mExposuresCleanupBefore,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_cleanup_delete_failed_count",
			Description: "Total count of exposures delete failed",
			Measure:     mExposuresDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_deleted_count",
			Description: "Total count of exposures deletions",
			Measure:     mExposuresDeleted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_stats_cleanup_delete_failed_count",
			Description: "Total count of exposures stats delete failed",
			Measure:     mExposuresStatsDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_stats_deleted_count",
			Description: "Total count of stats exposures deletions",
			Measure:     mExposuresStatsDeleted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_setup_failed_count",
			Description: "Total count of exports setup failures",
			Measure:     mExportsSetupFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_cleanup_before_latest",
			Description: "Last value of exports cleanup cutoff date",
			Measure:     mExportsCleanupBefore,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_delete_failed_count",
			Description: "Total count of export deletion failures",
			Measure:     mExportsDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_deleted_count",
			Description: "Total count of exports deletions",
			Measure:     mExportsDeleted,
			Aggregation: view.Sum(),
		},
	}...)
}
