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
