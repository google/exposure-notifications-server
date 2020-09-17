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
