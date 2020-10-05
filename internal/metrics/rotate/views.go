package rotate

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats/view"
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "revision_keys_created_count",
			Description: "Total count of revision key creation instances",
			Measure:     RevisionKeysCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "revision_keys_deleted_count",
			Description: "Total count of revision key deletion instances",
			Measure:     RevisionKeysDeleted,
			Aggregation: view.Sum(),
		},
	}...)
}
