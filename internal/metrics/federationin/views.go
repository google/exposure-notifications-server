package federationin

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats/view"
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "pull_invalid_request_count",
			Description: "Total count of errors in pulling query IDs",
			Measure:     PullInvalidRequest,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "pull_lock_contention_count",
			Description: "Total count of lock contention during pull operations",
			Measure:     PullLockContention,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "pull_insertions_latest",
			Description: "Last value of exposure insertions",
			Measure:     PullInserts,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "pull_revisions_latest",
			Description: "Last value of exposure revisions",
			Measure:     PullRevisions,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "pull_droped_latest",
			Description: "Last value of exposure droped",
			Measure:     PullDropped,
			Aggregation: view.LastValue(),
		},
	}...)
}
