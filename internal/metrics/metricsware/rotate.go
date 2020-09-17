package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/rotate"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordRevisionKeyCreation(ctx context.Context) {
	(*m.exporter).WriteInt("revision-keys-created", true, 1)
	stats.Record(ctx, rotate.RevisionKeysCreated.M(1))
}

func (m Middleware) RecordRevisionKeyDeletion(ctx context.Context, deletions int) {
	(*m.exporter).WriteInt("revision-keys-deleted", true, deletions)
	stats.Record(ctx, rotate.RevisionKeysDeleted.M(int64(deletions)))
}
