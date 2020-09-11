package publish

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats/view"
)

var (
	Views = []*view.View{
		{
			Name:        metrics.MetricRoot + "pha_not_authorized_count",
			Description: "Total count of authorization failures for the Public Health Authority",
			Measure:     HealthAuthorityNotAuthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "error_loading_app_count",
			Description: "Total count of errors in loading the authorized application",
			Measure:     ErrorLoadingAuthorizedApp,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "region_not_authorized_count",
			Description: "Total count of region not being authorized",
			Measure:     RegionNotAuthorized,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "region_not_specified_count",
			Description: "Total count of region not being specified",
			Measure:     RegionNotSpecified,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "verification_bypassed_count",
			Description: "Total count of health authority verification being bypassed",
			Measure:     VerificationBypassed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "bad_verification_count",
			Description: "Total count of bad health authority verification",
			Measure:     BadVerification,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "transform_fail_count",
			Description: "Total count of transformations failures of exposure entities",
			Measure:     TransformFail,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "revision_token_issue_count",
			Description: "Total count of revision token processing issues",
			Measure:     RevisionTokenIssue,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_insertions_count",
			Description: "Total count of exposure insertions",
			Measure:     ExposuresInserted,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_revisions_count",
			Description: "Total count of exposure revisions",
			Measure:     ExposuresRevised,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "exposures_drops_count",
			Description: "Total count of exposure drops",
			Measure:     ExposuresDropped,
			Aggregation: view.Sum(),
		},
		// v1 and v1alpha1
		{
			Name:        metrics.MetricRoot + "bad_json",
			Description: "Total count of bad JSON in incoming requests",
			Measure:     BadJSON,
			Aggregation: view.Sum(),
		},
		// v1 and v1alpha1
		{
			Name:        metrics.MetricRoot + "padding_failed",
			Description: "Total count of response padding failures",
			Measure:     PaddingFailed,
			Aggregation: view.Sum(),
		},
	}
)
