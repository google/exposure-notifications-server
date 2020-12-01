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

package export

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"go.opencensus.io/stats/view"
)

func init() {
	observability.CollectViews([]*view.View{
		{
			Name:        metrics.MetricRoot + "export_batcher_lock_contention_count",
			Description: "Total count of lock contention instances",
			Measure:     BatcherLockContention,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_failure_count",
			Description: "Total count of export batcher failures",
			Measure:     BatcherFailure,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batcher_no_work_count",
			Description: "Total count for instances of export batcher having no work",
			Measure:     BatcherNoWork,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_batches_created_count",
			Description: "Total count for number of export batches created",
			Measure:     BatcherCreated,
			Aggregation: view.Sum(),
		},
		{
			Name:        metrics.MetricRoot + "export_worker_bad_key_length_latest",
			Description: "Latest number of dropped keys caused by bad key length",
			Measure:     WorkerBadKeyLength,
			Aggregation: view.LastValue(),
		},
	}...)
}
