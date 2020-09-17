// Package publish contains OpenCensus metrics and views for publish operations
package publish

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	publishMetricsPrefix = metrics.MetricRoot + "publish/"

	HealthAuthorityNotAuthorized = stats.Int64(publishMetricsPrefix+"pha_not_authorized",
		"Public health authority authorized failures", stats.UnitDimensionless)
	ErrorLoadingAuthorizedApp = stats.Int64(publishMetricsPrefix+"error_loading_app",
		"Errors in loading authorized app", stats.UnitDimensionless)
	RegionNotAuthorized = stats.Int64(publishMetricsPrefix+"region_not_authorized",
		"Instances of region not being authorized", stats.UnitDimensionless)
	RegionNotSpecified = stats.Int64(publishMetricsPrefix+"region_not_specified",
		"Instances of region not being specified", stats.UnitDimensionless)
	VerificationBypassed = stats.Int64(publishMetricsPrefix+"verification_bypassed",
		"Instances of health authority verification being bypassed", stats.UnitDimensionless)
	BadVerification = stats.Int64(publishMetricsPrefix+"bad_verification",
		"Instances of bad health authority verification", stats.UnitDimensionless)
	TransformFail = stats.Int64(publishMetricsPrefix+"transform_fail",
		"Instances of failure fo transformation to exposure entities", stats.UnitDimensionless)
	RevisionTokenIssue = stats.Int64(publishMetricsPrefix+"revision_token_issue",
		"Instances of issues in revision token processing", stats.UnitDimensionless)
	ExposuresInserted = stats.Int64(publishMetricsPrefix+"exposures_insertions",
		"Instances of exposure being inserted", stats.UnitDimensionless)
	ExposuresRevised = stats.Int64(publishMetricsPrefix+"exposures_revisions",
		"Instances of exposure being revised", stats.UnitDimensionless)
	ExposuresDropped = stats.Int64(publishMetricsPrefix+"exposures_drops",
		"Instances of exposure being dropped", stats.UnitDimensionless)
	// v1 and v1alpha1
	BadJSON = stats.Int64(publishMetricsPrefix+"bad_json",
		"Instances of bad JSON in incoming request", stats.UnitDimensionless)
	// v1 and v1alpha1
	PaddingFailed = stats.Int64(publishMetricsPrefix+"padding_failed",
		"Instances of response padding failures", stats.UnitDimensionless)
)
