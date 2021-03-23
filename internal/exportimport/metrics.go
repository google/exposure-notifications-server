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
	"context"
	"strconv"

	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const metricPrefix = metrics.MetricRoot + "export-importer"

var exportimportConfigIDTagKey = tag.MustNewKey("export_importer_config_id")

var (
	// mImportSuccess is the overall success of the import job.
	mImportSuccess = stats.Int64(metricPrefix+"/import/success", "successful import execution", stats.UnitDimensionless)

	// mScheduleSuccess is the overall success of the scheduler job.
	mScheduleSuccess = stats.Int64(metricPrefix+"/schedule/success", "successful schedule execution", stats.UnitDimensionless)

	// Configuration specific metrics.
	mFilesScheduled = stats.Int64(metricPrefix+"/files_scheduled", "Number of import files scheduled by ID", stats.UnitDimensionless)
	mFilesImported  = stats.Int64(metricPrefix+"/files_imported", "Number of import files completed by ID", stats.UnitDimensionless)
	mFilesFailed    = stats.Int64(metricPrefix+"/files_failed", "Number of import files failed by ID", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/import/success",
			Description: "Number of import successes",
			Measure:     mImportSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/schedule/success",
			Description: "Number of scheduler successes",
			Measure:     mScheduleSuccess,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/files_scheduled",
			Description: "Total count of files scheduled, by configuration",
			Measure:     mFilesScheduled,
			Aggregation: view.Sum(),
			TagKeys:     metricsTagKeys(),
		},
		{
			Name:        metricPrefix + "/files_imported",
			Description: "Total count of files imported, by configuration",
			Measure:     mFilesImported,
			Aggregation: view.Sum(),
			TagKeys:     metricsTagKeys(),
		},
		{
			Name:        metricPrefix + "/files_failed",
			Description: "Total count of files failed, by configuration",
			Measure:     mFilesFailed,
			Aggregation: view.Sum(),
			TagKeys:     metricsTagKeys(),
		},
	}...)
}

func metricsTagKeys() []tag.Key {
	return []tag.Key{
		exportimportConfigIDTagKey,
	}
}

func metricsWithExportImportID(octx context.Context, id int64) context.Context {
	idStr := strconv.FormatInt(id, 10)
	ctx, err := tag.New(octx, tag.Upsert(exportimportConfigIDTagKey, idStr))
	if err != nil {
		logging.FromContext(octx).Named("metricsWithExportImportID").
			Errorw("failed upsert exportimport on context", "error", err, "export_import_id", id)
		return octx
	}
	return ctx
}
