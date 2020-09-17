// Package federationout contains OpenCensus metrics and views for federationout operations
package federationout

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	federationoutMetricsPrefix = metrics.MetricRoot + "federationout/"

	FetchFailed = stats.Int64(federationoutMetricsPrefix+"fetch_failed",
		"Error in fetching exposures", stats.UnitDimensionless)
	FetchRegionsRequested = stats.Int64(federationoutMetricsPrefix+"fetch_regions_requested",
		"Instances of fetch regions being requested", stats.UnitDimensionless)
	FetchRegionsExcluded = stats.Int64(federationoutMetricsPrefix+"fetch_regions_excluded",
		"Instances of fetch regions being excluded", stats.UnitDimensionless)
	FetchError = stats.Int64(federationoutMetricsPrefix+"fetch_error",
		"Instances of fetch errors", stats.UnitDimensionless)
	FetchCount = stats.Int64(federationoutMetricsPrefix+"fetch_count",
		"Fetch count value", stats.UnitDimensionless)
	FetchInvalidAuthToken = stats.Int64(federationoutMetricsPrefix+"fetch_invalid_auth_token",
		"Instances of invalid auth tokens during fetch operations", stats.UnitDimensionless)
	FetchUnauthorized = stats.Int64(federationoutMetricsPrefix+"fetch_unauthorized",
		"Instances of unauthorized fetch attempts", stats.UnitDimensionless)
	FetchInternalError = stats.Int64(federationoutMetricsPrefix+"fetch_internal_error",
		"Instances of internal errors during fetch attempts", stats.UnitDimensionless)
	FetchInvalidAudience = stats.Int64(federationoutMetricsPrefix+"fetch_invalid_audience",
		"Instances of invalid audiences for fetch operations", stats.UnitDimensionless)
)
