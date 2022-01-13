// Copyright 2021 the Exposure Notifications Server authors
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

package metricsregistrar

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"github.com/google/exposure-notifications-server/pkg/logging"
	"github.com/google/exposure-notifications-server/pkg/observability"
	"github.com/hashicorp/go-multierror"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/metric"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

// createMetrics creates the upstream metrics in Stackdriver.
func (s *Server) createMetrics(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.Infow("starting metric registration")
	defer logger.Infow("finished metric registration")

	// Extract the project ID.
	sd := s.config.ObservabilityExporter.Stackdriver
	if sd == nil {
		return fmt.Errorf("observability export is not stackdriver")
	}
	projectID := sd.ProjectID

	// Create the metrics client.
	client, err := monitoring.NewMetricClient(context.Background())
	if err != nil {
		return fmt.Errorf("failed to create metrics client: %w", err)
	}
	defer client.Close()

	// Get the list of descriptors that are already registered.
	iter := client.ListMetricDescriptors(context.Background(), &monitoringpb.ListMetricDescriptorsRequest{
		Name: "projects/" + projectID,
	})
	existingMetricDescriptors := make(map[string]*metric.MetricDescriptor)
	for {
		resp, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to list metrics: %w", err)
		}

		typ := resp.GetName()
		existingMetricDescriptors[typ] = resp
	}

	// Create the Stackdriver exporter.
	exporter, err := observability.NewStackdriver(context.Background(), &observability.StackdriverConfig{
		ProjectID: projectID,
	})
	if err != nil {
		return fmt.Errorf("failed to create Stackdriver exporter: %w", err)
	}

	// Register metric descriptors with Stackdriver.
	allViews := observability.AllViews()

	workers := int64(runtime.NumCPU())
	if workers < 3 {
		workers = 3
	}

	sem := semaphore.NewWeighted(workers)
	errCh := make(chan error, len(allViews))

	for _, view := range allViews {
		view := view

		if err := sem.Acquire(ctx, 1); err != nil {
			return fmt.Errorf("failed to acquire semaphore: %w", err)
		}

		go func() {
			defer sem.Release(1)

			metricDescriptor, err := exporter.ViewToMetricDescriptor(view)
			if err != nil {
				errCh <- fmt.Errorf("failed to convert view %s to MetricDescriptor: %w", view.Name, err)
				return
			}

			if existing, ok := existingMetricDescriptors[metricDescriptor.GetName()]; ok {
				if existing.GetType() == metricDescriptor.GetType() &&
					existing.GetMetricKind() == metricDescriptor.GetMetricKind() &&
					existing.GetValueType() == metricDescriptor.GetValueType() &&
					existing.GetUnit() == metricDescriptor.GetUnit() &&
					existing.GetDescription() == metricDescriptor.GetDescription() &&
					existing.GetDisplayName() == metricDescriptor.GetDisplayName() {
					logger.Debugw("skipping registration, already registered",
						"name", metricDescriptor.GetName(),
						"view", view.Name)
					return
				}

				// If we got this far, the metric exists, but it has invalid properties.
				// Delete the metric (there is no update option).
				logger.Warnw("metric is already registered, but properties differ, deleting",
					"name", metricDescriptor.GetName(),
					"existing", existing,
					"new", metricDescriptor)
				if err := client.DeleteMetricDescriptor(ctx, &monitoringpb.DeleteMetricDescriptorRequest{
					Name: metricDescriptor.GetName(),
				}); err != nil {
					errCh <- fmt.Errorf("failed to delete existing metric descriptor for view %q: %w", view.Name, err)
					// Don't return, still try to create the metric below
				}
			}

			logger.Infow("registering metrics descriptor",
				"name", metricDescriptor.GetName(),
				"view", view.Name,
				"descriptor", metricDescriptor)

			req := &monitoringpb.CreateMetricDescriptorRequest{
				Name:             "projects/" + projectID,
				MetricDescriptor: metricDescriptor,
			}

			ctx, done := context.WithTimeout(ctx, 10*time.Second)
			defer done()

			if _, err := client.CreateMetricDescriptor(ctx, req); err != nil {
				errCh <- fmt.Errorf("failed to create metric descriptor for view %s: %w", view.Name, err)
				return
			}
		}()
	}

	if err := sem.Acquire(ctx, workers); err != nil {
		return fmt.Errorf("failed to wait for semaphore: %w", err)
	}
	close(errCh)

	var merr *multierror.Error
	for err := range errCh {
		if err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	return merr.ErrorOrNil()
}
