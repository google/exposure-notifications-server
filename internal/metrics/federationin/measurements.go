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
