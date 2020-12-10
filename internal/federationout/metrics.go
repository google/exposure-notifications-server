// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package federationout contains OpenCensus metrics and views for federationout operations
package federationout

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	federationoutMetricsPrefix = metrics.MetricRoot + "federationout/"

	mFetchFailed = stats.Int64(federationoutMetricsPrefix+"fetch_failed",
		"Error in fetching exposures", stats.UnitDimensionless)
	mFetchRegionsRequested = stats.Int64(federationoutMetricsPrefix+"fetch_regions_requested",
		"Instances of fetch regions being requested", stats.UnitDimensionless)
	mFetchRegionsExcluded = stats.Int64(federationoutMetricsPrefix+"fetch_regions_excluded",
		"Instances of fetch regions being excluded", stats.UnitDimensionless)
	mFetchError = stats.Int64(federationoutMetricsPrefix+"fetch_error",
		"Instances of fetch errors", stats.UnitDimensionless)
	mFetchCount = stats.Int64(federationoutMetricsPrefix+"fetch_count",
		"Fetch count value", stats.UnitDimensionless)
	mFetchInvalidAuthToken = stats.Int64(federationoutMetricsPrefix+"fetch_invalid_auth_token",
		"Instances of invalid auth tokens during fetch operations", stats.UnitDimensionless)
	mFetchUnauthorized = stats.Int64(federationoutMetricsPrefix+"fetch_unauthorized",
		"Instances of unauthorized fetch attempts", stats.UnitDimensionless)
	mFetchInternalError = stats.Int64(federationoutMetricsPrefix+"fetch_internal_error",
		"Instances of internal errors during fetch attempts", stats.UnitDimensionless)
	mFetchInvalidAudience = stats.Int64(federationoutMetricsPrefix+"fetch_invalid_audience",
		"Instances of invalid audiences for fetch operations", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "fetch_failed_count",
			Description: "Total count of fetch failures",
			Measure:     mFetchFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_regions_requested_latest",
			Description: "Last value of fetch region requests",
			Measure:     mFetchRegionsRequested,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_regions_excluded_latest",
			Description: "Last value of fetch region exclusions",
			Measure:     mFetchRegionsExcluded,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_error_count",
			Description: "Total count of fetch errors",
			Measure:     mFetchError,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_count_latest",
			Description: "Latest value of fetch counts",
			Measure:     mFetchCount,
			Aggregation: view.LastValue(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_invalid_auth_token_count",
			Description: "Total count fo invalid auth tokens during fetch operations",
			Measure:     mFetchInvalidAuthToken,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_unauthorized_count",
			Description: "Total count of unauthorized fetch attempts",
			Measure:     mFetchUnauthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_internal_error_count",
			Description: "Total count of internal errors during fetch attempts",
			Measure:     mFetchInternalError,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "fetch_invalid_audience_count",
			Description: "Total count of invalid audience errors during fetch operations",
			Measure:     mFetchInvalidAudience,
			Aggregation: view.Sum(),
		},
	}...)
}
