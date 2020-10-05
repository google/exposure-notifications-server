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
