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

package cleanup

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats/view"
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "exposures_setup_failed_count",
			Description: "Total count of exposures setup failures",
			Measure:     ExposuresSetupFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_cleanup_before_latest",
			Description: "Last value of exposures cleanup cutoff date",
			Measure:     ExposuresCleanupBefore,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_cleanup_delete_failed_count",
			Description: "Total count of exposures delete failed",
			Measure:     ExposuresDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_deleted_count",
			Description: "Total count of exposures deletions",
			Measure:     ExposuresDeleted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposure_stats_cleanup_delete_failed_count",
			Description: "Total count of exposures stats delete failed",
			Measure:     ExposuresStatsDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_stats_deleted_count",
			Description: "Total count of stats exposures deletions",
			Measure:     ExposuresStatsDeleted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_setup_failed_count",
			Description: "Total count of exports setup failures",
			Measure:     ExportsSetupFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_cleanup_before_latest",
			Description: "Last value of exports cleanup cutoff date",
			Measure:     ExportsCleanupBefore,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_delete_failed_count",
			Description: "Total count of export deletion failures",
			Measure:     ExportsDeleteFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exports_deleted_count",
			Description: "Total count of exports deletions",
			Measure:     ExportsDeleted,
			Aggregation: view.Sum(),
		},
	}...)
}
