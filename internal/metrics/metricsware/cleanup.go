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
