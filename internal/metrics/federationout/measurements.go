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
