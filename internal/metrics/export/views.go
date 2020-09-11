package export

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats/view"
)

var (
	Views = []*view.View{
		{
			Name:        metrics.MetricRoot + "export_batcher_lock_contention_count",
			Description: "Total count of lock contention instances",
			Measure:     BatcherLockContention,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_failure_count",
			Description: "Total count of export batcher failures",
			Measure:     BatcherFailure,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_no_work_count",
			Description: "Total count for instances of export batcher having no work",
			Measure:     BatcherNoWork,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batches_created_count",
			Description: "Total count for number of export batches created",
			Measure:     BatcherCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_worker_bad_key_length_latest",
			Description: "Latest number of dropped keys caused by bad key length",
			Measure:     WorkerBadKeyLength,
			Aggregation: view.LastValue(),
		},
	}
)
