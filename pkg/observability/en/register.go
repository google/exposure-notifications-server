// Package en provide metric registration logic specific to the notification server.
package en

import (
	"fmt"

	"github.com/google/exposure-notifications-server/internal/metrics/cleanup"
	"github.com/google/exposure-notifications-server/internal/metrics/export"
	"github.com/google/exposure-notifications-server/internal/metrics/federationin"
	"github.com/google/exposure-notifications-server/internal/metrics/federationout"
	"github.com/google/exposure-notifications-server/internal/metrics/publish"
	"github.com/google/exposure-notifications-server/internal/metrics/rotate"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

// RegisterViews registers the necessary tracing views.
func RegisterViews() error {
	// Record the various HTTP view to collect metrics.
	httpViews := append(ochttp.DefaultServerViews, ochttp.DefaultClientViews...)
	if err := view.Register(httpViews...); err != nil {
		return fmt.Errorf("failed to register http views: %w", err)
	}

	// Register the various gRPC views to collect metrics.
	gRPCViews := append(ocgrpc.DefaultServerViews, ocgrpc.DefaultClientViews...)
	if err := view.Register(gRPCViews...); err != nil {
		return fmt.Errorf("failed to register grpc views: %w", err)
	}

	if err := view.Register(cleanup.Views...); err != nil {
		return fmt.Errorf("failed to register cleanup metrics: %w", err)
	}

	if err := view.Register(export.Views...); err != nil {
		return fmt.Errorf("failed to register export metrics: %w", err)
	}

	if err := view.Register(federationin.Views...); err != nil {
		return fmt.Errorf("failed to register federationin metrics: %w", err)
	}

	if err := view.Register(federationout.Views...); err != nil {
		return fmt.Errorf("failed to register federationout metrics: %w", err)
	}

	if err := view.Register(publish.Views...); err != nil {
		return fmt.Errorf("failed to register publish metrics: %w", err)
	}

	if err := view.Register(rotate.Views...); err != nil {
		return fmt.Errorf("failed to register rotate metrics: %w", err)
	}

	return nil
}
