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

	"github.com/google/exposure-notifications-server/internal/metrics/export"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordExportBatcherLockContention(ctx context.Context) {
	(*m.exporter).WriteInt("export-export-lock-contention", true, 1)
	stats.Record(ctx, export.BatcherLockContention.M(1))
}

func (m Middleware) RecordExportBatcherFailure(ctx context.Context) {
	(*m.exporter).WriteInt("export-export-failed", true, 1)
	stats.Record(ctx, export.BatcherFailure.M(1))
}

func (m Middleware) RecordExportBatcherNoWork(ctx context.Context) {
	(*m.exporter).WriteInt("export-export-no-work", true, 1)
	stats.Record(ctx, export.BatcherNoWork.M(1))
}

func (m Middleware) RecordExportBatcherCreation(ctx context.Context, batches int) {
	(*m.exporter).WriteInt("export-export-created", true, batches)
	stats.Record(ctx, export.BatcherCreated.M(int64(batches)))
}

func (m Middleware) RecordExportWorkerBadKeyLength(ctx context.Context, droppedKeys int) {
	(*m.exporter).WriteInt("export-bad-key-length", false, droppedKeys)
	stats.Record(ctx, export.WorkerBadKeyLength.M(int64(droppedKeys)))
}
