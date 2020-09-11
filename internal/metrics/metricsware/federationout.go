package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/cleanup"
	"github.com/google/exposure-notifications-server/internal/metrics/federationout"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordFetchFailure(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-failed", true, 1)
	stats.Record(ctx, federationout.FetchFailed.M(1))
}

func (m Middleware) RecordFetchRegionsRequested(ctx context.Context, requestedRegions int) {
	(*m.exporter).WriteInt("federation-fetch-regions-requested", false, requestedRegions)
	stats.Record(ctx, federationout.FetchRegionsRequested.M(int64(requestedRegions)))
}

func (m Middleware) RecordFetchRegionsExcluded(ctx context.Context, excludedRegions int) {
	(*m.exporter).WriteInt("federation-fetch-regions-excluded", false, excludedRegions)
	stats.Record(ctx, federationout.FetchRegionsExcluded.M(int64(excludedRegions)))
}

func (m Middleware) RecordFetchError(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-error", true, 1)
	stats.Record(ctx, federationout.FetchError.M(1))
}

func (m Middleware) RecordFetchCount(ctx context.Context, count int) {
	(*m.exporter).WriteInt("federation-fetch-count", false, count)
	stats.Record(ctx, federationout.FetchCount.M(int64(count)))
}

func (m Middleware) RecordInvalidFetchAuthToken(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-invalid-auth-token", true, 1)
	stats.Record(ctx, federationout.FetchInvalidAuthToken.M(1))
}

func (m Middleware) RecordUnauthorizedFetchAttempt(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-unauthorized", true, 1)
	stats.Record(ctx, federationout.FetchUnauthorized.M(1))
}

func (m Middleware) RecordInternalErrorDuringFetch(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-internal-error", true, 1)
	stats.Record(ctx, federationout.FetchInternalError.M(1))
}

func (m Middleware) RecordInvalidAudienceDuringFetch(ctx context.Context) {
	(*m.exporter).WriteInt("federation-fetch-invalid-audience", true, 1)
	stats.Record(ctx, federationout.FetchInvalidAudience.M(1))
}

func (m Middleware) RecordExportsCutoffDate(ctx context.Context, cutoff int64) {
	(*m.exporter).WriteInt64("cleanup-exports-before", false, cutoff)
	stats.Record(ctx, cleanup.ExportsCleanupBefore.M(cutoff))
}

func (m Middleware) RecordExportsDeleteFailed(ctx context.Context) {
	(*m.exporter).WriteInt("cleanup-exports-delete-failed", true, 1)
	stats.Record(ctx, cleanup.ExportsDeleteFailed.M(1))
}

func (m Middleware) RecordExportsDeleted(ctx context.Context, deletions int) {
	(*m.exporter).WriteInt("cleanup-exports-deleted", true, deletions)
	stats.Record(ctx, cleanup.ExportsDeleted.M(int64(deletions)))
}
