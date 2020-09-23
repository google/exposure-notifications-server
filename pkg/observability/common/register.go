package common

import (
	"fmt"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

// RegisterViews registers the most common views with OpenCensus.
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
	return nil
}
