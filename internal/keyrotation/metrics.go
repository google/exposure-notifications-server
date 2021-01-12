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

// Package keyrotation contains OpenCensus metrics and views for rotate operations
package keyrotation

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	rotateMetricsPrefix = metrics.MetricRoot + "rotate"

	mRevisionKeysCreated = stats.Int64(rotateMetricsPrefix+"revision_keys_created",
		"Instance of revision key being created", stats.UnitDimensionless)
	mRevisionKeysDeleted = stats.Int64(rotateMetricsPrefix+"revision_keys_deleted",
		"Instance of revision keys being deleted", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "revision_keys_created_count",
			Description: "Total count of revision key creation instances",
			Measure:     mRevisionKeysCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "revision_keys_deleted_count",
			Description: "Total count of revision key deletion instances",
			Measure:     mRevisionKeysDeleted,
			Aggregation: view.Sum(),
		},
	}...)
}
