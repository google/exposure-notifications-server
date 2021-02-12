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

package observability

import (
	"context"
	"strings"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"github.com/google/uuid"
)

var _ monitoredresource.Interface = (*stackdriverMonitoredResource)(nil)

type stackdriverMonitoredResource struct {
	Resource string
	Labels   map[string]string
}

// NewStackdriverMonitoredResource returns a monitored resource with the
// required labels filled out. This needs to be the correct resource type so we
// can compared the default stackdriver metrics with the custom metrics we're
// generating.
//
// NOTE: This code is focused on support GCP Cloud Run Managed. If you are
// running in a different environment, you may see weird results.
func NewStackdriverMonitoredResource(ctx context.Context, c *StackdriverConfig) monitoredresource.Interface {
	logger := logging.FromContext(ctx).Named("stackdriver")

	resource := "generic_task"
	labels := map[string]string{}

	labels["project_id"] = c.ProjectID

	labels["job"] = c.Service
	if labels["job"] == "" {
		labels["job"] = "unknown"
	}

	// Try to get task_id from metadata server.
	iid, err := metadata.InstanceID()
	if err != nil {
		logger.Errorw("could not get instance id", "error", err)
	}
	labels["task_id"] = iid

	// Worse case task_id
	if labels["task_id"] == "" {
		labels["task_id"] = uuid.New().String()
	}

	region, err := metadata.Get("instance/region")
	if err != nil {
		logger.Errorw("could not get region", "error", err)
		labels["location"] = "unknown"
	}
	labels["location"] = region

	// Metadata often returns the following: "projects/111111111111/regions/us-central1"
	pieces := strings.Split(region, "/")
	if len(pieces) == 4 {
		labels["location"] = pieces[3]
	} else {
		logger.Errorw("region did not match expected format", "region", region)
	}

	labels["namespace"] = c.Namespace

	filteredLabels := removeUnusedLabels(resource, labels)

	return &stackdriverMonitoredResource{
		Resource: resource,
		Labels:   filteredLabels,
	}
}

func (s *stackdriverMonitoredResource) MonitoredResource() (string, map[string]string) {
	return s.Resource, s.Labels
}

// removeUnusedLabels deletes unused labels to not flood stackdriver.
func removeUnusedLabels(resource string, in map[string]string) map[string]string {
	// The labels each resource type requires.
	requiredLabels := map[string]map[string]bool{
		// https://cloud.google.com/monitoring/api/resources#tag_generic_task
		"generic_task": {"project_id": true, "location": true, "namespace": true, "job": true, "task_id": true},
	}

	ret := map[string]string{}
	for k, v := range in {
		if requiredLabels[resource][k] {
			ret[k] = v
		}
	}

	return ret
}
