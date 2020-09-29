package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/federationin"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordPullInvalidRequest(ctx context.Context) {
	(*m.exporter).WriteInt("federation-pull-invalid-request", true, 1)
	stats.Record(ctx, federationin.PullInvalidRequest.M(1))
}

func (m Middleware) RecordPullLockContention(ctx context.Context) {
	(*m.exporter).WriteInt("federation-pull-lock-contention", true, 1)
	stats.Record(ctx, federationin.PullLockContention.M(1))
}

func (m Middleware) RecordPullInsertions(ctx context.Context, numExposures int) {
	(*m.exporter).WriteInt("federation-pull-inserts", false, numExposures)
	stats.Record(ctx, federationin.PullInserts.M(int64(numExposures)))
}

func (m Middleware) RecordPullRevisions(ctx context.Context, numRevised int) {
	(*m.exporter).WriteInt("federation-pull-revisions", false, numRevised)
	stats.Record(ctx, federationin.PullRevisions.M(int64(numRevised)))
}

func (m Middleware) RecordPullDropped(ctx context.Context, numDroped int) {
	(*m.exporter).WriteInt("federation-pull-droped", false, numDroped)
	stats.Record(ctx, federationin.PullDropped.M(int64(numDroped)))
}
