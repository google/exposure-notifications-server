// Package federationin contains OpenCensus metrics and views for federationin operations
package federationin

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	publishMetricsPrefix = metrics.MetricRoot + "federationin/"
	PullInvalidRequest   = stats.Int64(publishMetricsPrefix+"pull_invalid_request",
		"Invalid query IDs in pull operation", stats.UnitDimensionless)
	PullLockContention = stats.Int64(publishMetricsPrefix+"pull_lock_contention",
		"Lock contention during pull operation", stats.UnitDimensionless)
	PullInserts = stats.Int64(publishMetricsPrefix+"pull_insertions",
		"Pull insertion", stats.UnitDimensionless)
	PullRevisions = stats.Int64(publishMetricsPrefix+"pull_revision",
		"Pull revision", stats.UnitDimensionless)
	PullDropped = stats.Int64(publishMetricsPrefix+"pull_dropped",
		"Pull dropped", stats.UnitDimensionless)
)
