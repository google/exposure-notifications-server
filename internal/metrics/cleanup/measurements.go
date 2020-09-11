// Package cleanup contains OpenCensus metrics and views for cleanup operations
package cleanup

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	cleanupMetricsPrefix = metrics.MetricRoot + "cleanup"

	ExposuresSetupFailed = stats.Int64(cleanupMetricsPrefix+"exposures_setup_failed",
		"Instances of exposures setup failures", stats.UnitDimensionless)
	ExposuresCleanupBefore = stats.Int64(cleanupMetricsPrefix+"exposures_cleanup_before",
		"Exposures cleanup cutoff date", stats.UnitSeconds)
	ExposuresDeleteFailed = stats.Int64(cleanupMetricsPrefix+"exposures_delete_failed",
		"Instances of exposures delete failures", stats.UnitDimensionless)
	ExposuresDeleted = stats.Int64(cleanupMetricsPrefix+"exposures_deleted",
		"Exposures deletions", stats.UnitDimensionless)
	ExportsSetupFailed = stats.Int64(cleanupMetricsPrefix+"exports_setup_failed",
		"Instances of export setup failures", stats.UnitDimensionless)
	ExportsCleanupBefore = stats.Int64(cleanupMetricsPrefix+"exports_cleanup_before",
		"Exports cleanup cutoff date", stats.UnitSeconds)
	ExportsDeleteFailed = stats.Int64(cleanupMetricsPrefix+"exports_delete_failed",
		"Instances of exports delete failures", stats.UnitDimensionless)
	ExportsDeleted = stats.Int64(cleanupMetricsPrefix+"exports_deleted",
		"Exports deletions", stats.UnitDimensionless)
)
