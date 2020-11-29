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

// Package rotate contains OpenCensus metrics and views for rotate operations
package rotate

import (
	"github.com/google/exposure-notifications-server/internal/metrics"
	"go.opencensus.io/stats"
)

var (
	rotateMetricsPrefix = metrics.MetricRoot + "rotate"

	RevisionKeysCreated = stats.Int64(rotateMetricsPrefix+"revision_keys_created",
		"Instance of revision key being created", stats.UnitDimensionless)
	RevisionKeysDeleted = stats.Int64(rotateMetricsPrefix+"revision_keys_deleted",
		"Instance of revision keys being deleted", stats.UnitDimensionless)
)
