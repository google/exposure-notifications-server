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

// Package publish contains OpenCensus metrics and views for publish operations
package publish

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	publishMetricsPrefix = metrics.MetricRoot + "publish/"

	mHealthAuthorityNotAuthorized = stats.Int64(publishMetricsPrefix+"pha_not_authorized",
		"Public health authority authorized failures", stats.UnitDimensionless)
	mErrorLoadingAuthorizedApp = stats.Int64(publishMetricsPrefix+"error_loading_app",
		"Errors in loading authorized app", stats.UnitDimensionless)
	mRegionNotAuthorized = stats.Int64(publishMetricsPrefix+"region_not_authorized",
		"Instances of region not being authorized", stats.UnitDimensionless)
	mRegionNotSpecified = stats.Int64(publishMetricsPrefix+"region_not_specified",
		"Instances of region not being specified", stats.UnitDimensionless)
	mVerificationBypassed = stats.Int64(publishMetricsPrefix+"verification_bypassed",
		"Instances of health authority verification being bypassed", stats.UnitDimensionless)
	mBadVerification = stats.Int64(publishMetricsPrefix+"bad_verification",
		"Instances of bad health authority verification", stats.UnitDimensionless)
	mTransformFail = stats.Int64(publishMetricsPrefix+"transform_fail",
		"Instances of failure fo transformation to exposure entities", stats.UnitDimensionless)
	mRevisionTokenIssue = stats.Int64(publishMetricsPrefix+"revision_token_issue",
		"Instances of issues in revision token processing", stats.UnitDimensionless)
	mExposuresInserted = stats.Int64(publishMetricsPrefix+"exposures_insertions",
		"Instances of exposure being inserted", stats.UnitDimensionless)
	mExposuresRevised = stats.Int64(publishMetricsPrefix+"exposures_revisions",
		"Instances of exposure being revised", stats.UnitDimensionless)
	mExposuresDropped = stats.Int64(publishMetricsPrefix+"exposures_drops",
		"Instances of exposure being dropped", stats.UnitDimensionless)
	// v1 and v1alpha1
	mBadJSON = stats.Int64(publishMetricsPrefix+"bad_json",
		"Instances of bad JSON in incoming request", stats.UnitDimensionless)
	// v1 and v1alpha1
	mPaddingFailed = stats.Int64(publishMetricsPrefix+"padding_failed",
		"Instances of response padding failures", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "pha_not_authorized_count",
			Description: "Total count of authorization failures for the Public Health Authority",
			Measure:     mHealthAuthorityNotAuthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "error_loading_app_count",
			Description: "Total count of errors in loading the authorized application",
			Measure:     mErrorLoadingAuthorizedApp,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "region_not_authorized_count",
			Description: "Total count of region not being authorized",
			Measure:     mRegionNotAuthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "region_not_specified_count",
			Description: "Total count of region not being specified",
			Measure:     mRegionNotSpecified,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "verification_bypassed_count",
			Description: "Total count of health authority verification being bypassed",
			Measure:     mVerificationBypassed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "bad_verification_count",
			Description: "Total count of bad health authority verification",
			Measure:     mBadVerification,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "transform_fail_count",
			Description: "Total count of transformations failures of exposure entities",
			Measure:     mTransformFail,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "revision_token_issue_count",
			Description: "Total count of revision token processing issues",
			Measure:     mRevisionTokenIssue,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_insertions_count",
			Description: "Total count of exposure insertions",
			Measure:     mExposuresInserted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_revisions_count",
			Description: "Total count of exposure revisions",
			Measure:     mExposuresRevised,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_drops_count",
			Description: "Total count of exposure drops",
			Measure:     mExposuresDropped,
			Aggregation: view.Sum(),
		},
		// v1 and v1alpha1
		{
			Name:        metrics.MetricRoot + "bad_json",
			Description: "Total count of bad JSON in incoming requests",
			Measure:     mBadJSON,
			Aggregation: view.Sum(),
		},
		// v1 and v1alpha1
		{
			Name:        metrics.MetricRoot + "padding_failed",
			Description: "Total count of response padding failures",
			Measure:     mPaddingFailed,
			Aggregation: view.Sum(),
		},
	}...)
}
