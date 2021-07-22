// Copyright 2020 the Exposure Notifications Server authors
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

	mJWTNotYetValid = stats.Int64(publishMetricsPrefix+"jwt_not_yet_valid",
		"a certificate was presented with an IAT or NBF time that is in the past according to thie server", stats.UnitDimensionless)

	mNoPublicKey = stats.Int64(publishMetricsPrefix+"no_public_keys",
		"uploads where there is no public key", stats.UnitDimensionless)

	mExposuresCount = stats.Int64(publishMetricsPrefix+"exposures_count",
		"exposure count", stats.UnitDimensionless)

	mVerificationBypassed = stats.Int64(publishMetricsPrefix+"verification_bypassed",
		"Instances of health authority verification being bypassed", stats.UnitDimensionless)
	// v1 and v1alpha1
	mPaddingFailed = stats.Int64(publishMetricsPrefix+"padding_failed",
		"Instances of response padding failures", stats.UnitDimensionless)

	exposureTypeTag = tag.MustNewKey("type")

	requestTagKeys = []tag.Key{
		observability.BuildIDTagKey,
		observability.BuildTagTagKey,
		observability.BlameTagKey,
		observability.ResultTagKey,
	}
	exposureTagKeys = []tag.Key{
		observability.BuildIDTagKey,
		observability.BuildTagTagKey,
		exposureTypeTag,
	}

	healthAuthorityIDTag = tag.MustNewKey("healthAuthorityID")
	missingPublicKeyTags = []tag.Key{
		healthAuthorityIDTag,
	}
)

func exposureType(s string) tag.Mutator {
	return tag.Upsert(exposureTypeTag, s)
}

var (
	exposuresInserted = exposureType("INSERTED")
	exposuresRevised  = exposureType("REVISED")
	exposuresDropped  = exposureType("DROPPED")
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "requests_count",
			Description: "Total count of publish requests",
			Measure:     mLatencyMs,
			Aggregation: view.Count(),
			TagKeys:     requestTagKeys,
		},
		{
			Name:        metrics.MetricRoot + "requests_latency",
			Description: "Latency distribution of publish requests",
			Measure:     mLatencyMs,
			Aggregation: ochttp.DefaultLatencyDistribution,
			TagKeys:     requestTagKeys,
		},
		{
			Name:        metrics.MetricRoot + "exposures_count",
			Description: "Total count of published exposures.",
			Measure:     mExposuresCount,
			Aggregation: view.Count(),
			TagKeys:     exposureTagKeys,
		},
		{
			Name:        metrics.MetricRoot + "verification_bypassed_count",
			Description: "Total count of health authority verification being bypassed",
			Measure:     mVerificationBypassed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "jwt_not_yet_valid",
			Description: "Total count of instances where a verification certificate is in the future",
			Measure:     mJWTNotYetValid,
			Aggregation: view.Sum(),
			TagKeys:     missingPublicKeyTags,
		},
		// v1 and v1alpha1
		{
			Name:        metrics.MetricRoot + "padding_failed",
			Description: "Total count of response padding failures",
			Measure:     mPaddingFailed,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "no_public_key",
			Description: "Publish request with no public key",
			Measure:     mNoPublicKey,
			Aggregation: view.Count(),
			TagKeys:     missingPublicKeyTags,
		},
	}...)
}
