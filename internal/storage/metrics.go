// Copyright 2021 the Exposure Notifications Server authors
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

package storage

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

const metricPrefix = metrics.MetricRoot + "storage"

var (
	mAzureRefreshFailed  = stats.Int64(metricPrefix+"/azure/refresh_failed", "refresh token failed", stats.UnitDimensionless)
	mAzureRefreshExpired = stats.Int64(metricPrefix+"/azure/refresh_expired", "refresh token expired", stats.UnitDimensionless)
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metricPrefix + "/azure/refresh_failed",
			Description: "Number of failed refreshes",
			Measure:     mAzureRefreshFailed,
			Aggregation: view.Count(),
		},
		{
			Name:        metricPrefix + "/azure/refresh_expired",
			Description: "Number of refreshes that failed due to expiration",
			Measure:     mAzureRefreshExpired,
			Aggregation: view.Count(),
		},
	}...)
}
