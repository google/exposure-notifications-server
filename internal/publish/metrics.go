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
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	publishMetricsPrefix = metrics.MetricRoot + "publish/"

	mLatencyMs = stats.Float64(publishMetricsPrefix+"requests", "publish requests", stats.UnitMilliseconds)

	mVerificationBypassed = stats.Int64(publishMetricsPrefix+"verification_bypassed",
		"Instances of health authority verification being bypassed", stats.UnitDimensionless)
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

	tagKeys = []tag.Key{
		observability.BuildIDTagKey,
		observability.BuildTagTagKey,
		observability.BlameTagKey,
		observability.ResultTagKey,
	}
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "requests_count",
			Description: "Total count of publish requests",
			Measure:     mLatencyMs,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		{
			Name:        metrics.MetricRoot + "requests_latency",
			Description: "Latency distribution of publish requests",
			Measure:     mLatencyMs,
			Aggregation: ochttp.DefaultLatencyDistribution,
			TagKeys:     tagKeys,
		},
		{
			Name:        metrics.MetricRoot + "verification_bypassed_count",
			Description: "Total count of health authority verification being bypassed",
			Measure:     mVerificationBypassed,
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
