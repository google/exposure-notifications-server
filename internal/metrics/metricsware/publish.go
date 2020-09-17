package metricsware

import (
	"context"

	"github.com/google/exposure-notifications-server/internal/metrics/publish"
	"go.opencensus.io/stats"
)

func (m Middleware) RecordHealthAuthorityNotAuthorized(ctx context.Context) {
	(*m.exporter).WriteInt("publish-health-authority-not-authorized", true, 1)
	stats.Record(ctx, publish.HealthAuthorityNotAuthorized.M(1))
}

func (m Middleware) RecordErrorLoadingAuthorizedApp(ctx context.Context) {
	(*m.exporter).WriteInt("publish-error-loading-authorizedapp", true, 1)
	stats.Record(ctx, publish.ErrorLoadingAuthorizedApp.M(1))
}

func (m Middleware) RecordRegionNotAuthorized(ctx context.Context) {
	(*m.exporter).WriteInt("publish-region-not-authorized", true, 1)
	stats.Record(ctx, publish.RegionNotAuthorized.M(1))
}

func (m Middleware) RecordRegionNotSpecified(ctx context.Context) {
	(*m.exporter).WriteInt("publish-region-not-specified", true, 1)
	stats.Record(ctx, publish.RegionNotSpecified.M(1))
}

func (m Middleware) RecordVerificationBypassed(ctx context.Context) {
	(*m.exporter).WriteInt("publish-health-authority-verification-bypassed", true, 1)
	stats.Record(ctx, publish.VerificationBypassed.M(1))
}

func (m Middleware) RecordBadVerification(ctx context.Context) {
	(*m.exporter).WriteInt("publish-bad-verification", true, 1)
	stats.Record(ctx, publish.BadVerification.M(1))
}

func (m Middleware) RecordTransformFail(ctx context.Context) {
	(*m.exporter).WriteInt("publish-transform-fail", true, 1)
	stats.Record(ctx, publish.TransformFail.M(1))
}

func (m Middleware) RecordRevisionTokenIssue(ctx context.Context, metric string) {
	(*m.exporter).WriteInt(metric, true, 1)
	stats.Record(ctx, publish.RevisionTokenIssue.M(1))
}

func (m Middleware) RecordInsertions(ctx context.Context, insertions int) {
	(*m.exporter).WriteInt("publish-exposures-inserted", true, insertions)
	stats.Record(ctx, publish.ExposuresInserted.M(int64(insertions)))
}

func (m Middleware) RecordRevisions(ctx context.Context, revisions int) {
	(*m.exporter).WriteInt("publish-exposures-revised", true, revisions)
	stats.Record(ctx, publish.ExposuresInserted.M(int64(revisions)))
}

func (m Middleware) RecordDrops(ctx context.Context, drops int) {
	(*m.exporter).WriteInt("publish-exposures-dropped", true, drops)
	stats.Record(ctx, publish.ExposuresInserted.M(int64(drops)))
}

func (m Middleware) RecordBadJSON(ctx context.Context) {
	(*m.exporter).WriteInt("publish-bad-json", true, 1)
	stats.Record(ctx, publish.BadJSON.M(1))
}

func (m Middleware) RecordPaddingFailure(ctx context.Context) {
	(*m.exporter).WriteInt("padding-failed", true, 1)
	stats.Record(ctx, publish.PaddingFailed.M(1))
}
