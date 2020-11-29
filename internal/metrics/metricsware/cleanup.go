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

package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/cleanup"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordExposuresSetupFailed(ctx context.Context) {
	(*m.exporter).WriteInt("cleanup-exposures-setup-failed", true, 1)
	stats.Record(ctx, cleanup.ExposuresSetupFailed.M(1))
}

func (m Middleware) RecordExposuresCleanupCutoff(ctx context.Context, cutoff int64) {
	(*m.exporter).WriteInt64("cleanup-exposures-before", false, cutoff)
	stats.Record(ctx, cleanup.ExposuresCleanupBefore.M(cutoff))
}

func (m Middleware) RecordExposuresDeleteFailed(ctx context.Context) {
	(*m.exporter).WriteInt("cleanup-exposures-delete-failed", true, 1)
	stats.Record(ctx, cleanup.ExposuresDeleteFailed.M(1))
}

func (m Middleware) RecordExposuresDeleted(ctx context.Context, count int64) {
	(*m.exporter).WriteInt64("cleanup-exposures-deleted", true, count)
	stats.Record(ctx, cleanup.ExposuresDeleted.M(count))
}

func (m Middleware) RecordExportsSetupFailed(ctx context.Context) {
	(*m.exporter).WriteInt("cleanup-exports-setup-failed", true, 1)
	stats.Record(ctx, cleanup.ExportsSetupFailed.M(1))
}
