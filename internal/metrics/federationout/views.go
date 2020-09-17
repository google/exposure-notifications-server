package federationout

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats/view"
)

var (
	Views = []*view.View{
		{
			Name:        metrics.MetricRoot + "fetch_failed_count",
			Description: "Total count of fetch failures",
			Measure:     FetchFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_regions_requested_latest",
			Description: "Last value of fetch region requests",
			Measure:     FetchRegionsRequested,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_regions_excluded_latest",
			Description: "Last value of fetch region exclusions",
			Measure:     FetchRegionsExcluded,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_error_count",
			Description: "Total count of fetch errors",
			Measure:     FetchError,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_count_latest",
			Description: "Latest value of fetch counts",
			Measure:     FetchCount,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_invalid_auth_token_count",
			Description: "Total count fo invalid auth tokens during fetch operations",
			Measure:     FetchInvalidAuthToken,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_unauthorized_count",
			Description: "Total count of unauthorized fetch attempts",
			Measure:     FetchUnauthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_internal_error_count",
			Description: "Total count of internal errors during fetch attempts",
			Measure:     FetchInternalError,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_invalid_audience_count",
			Description: "Total count of invalid audience errors during fetch operations",
			Measure:     FetchInvalidAudience,
			Aggregation: view.Sum(),
		},
	}
)
