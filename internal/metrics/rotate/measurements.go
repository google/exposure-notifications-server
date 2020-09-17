// Package rotate contains OpenCensus metrics and views for rotate operations
package rotate

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	rotateMetricsPrefix = metrics.MetricRoot + "rotate"

	RevisionKeysCreated = stats.Int64(rotateMetricsPrefix+"revision_keys_created",
		"Instance of revision key being created", stats.UnitDimensionless)
	RevisionKeysDeleted = stats.Int64(rotateMetricsPrefix+"revision_keys_deleted",
		"Instance of revision keys being deleted", stats.UnitDimensionless)
)
