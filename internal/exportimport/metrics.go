// Copyright 2021 Google LLC
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

package exportimport

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	exportImportMetricsPrefix = metrics.MetricRoot + "exportimport"

	// Configuration specific metrics.
	mFilesScheduled = stats.Int64(exportImportMetricsPrefix+"files_scheduled",
		"Number of import files scheduled by ID", stats.UnitDimensionless)
	mFilesImported = stats.Int64(exportImportMetricsPrefix+"files_imported",
		"Number of import files completed by ID", stats.UnitDimensionless)
	mFilesFailed = stats.Int64(exportImportMetricsPrefix+"files_failed",
		"Number of import files failed by ID", stats.UnitDimensionless)

	// Job specific metrics.
	mScheduleCompletion = stats.Int64(exportImportMetricsPrefix+"schedule_completion",
		"Number of times the export-import scheduler completes", stats.UnitDimensionless)
	mImportCompletion = stats.Int64(exportImportMetricsPrefix+"import_completion",
		"Number of times the export-import job completes", stats.UnitDimensionless)

	exportImportIDTagKey = tag.MustNewKey("export_import_id")
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "exportimport_files_scheduled",
			Description: "Total count of files scheduled, by configuration",
			Measure:     mFilesScheduled,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{exportImportIDTagKey},
		},
		{
			Name:        metrics.MetricRoot + "exportimport_files_imported",
			Description: "Total count of files imported, by configuration",
			Measure:     mFilesImported,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{exportImportIDTagKey},
		},
		{
			Name:        metrics.MetricRoot + "exportimport_files_failed",
			Description: "Total count of files failed, by configuration",
			Measure:     mFilesFailed,
			Aggregation: view.Sum(),
			TagKeys:     []tag.Key{exportImportIDTagKey},
		},
		{
			Name:        metrics.MetricRoot + "exportimport_schedule_completion",
			Description: "Total count of export-import schedule job completion",
			Measure:     mScheduleCompletion,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exportimport_import_completion",
			Description: "Total count of export-import import job completion",
			Measure:     mImportCompletion,
			Aggregation: view.Sum(),
		},
	}...)
}
