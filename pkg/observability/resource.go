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

	"github.com/google/exposure-notifications-server/pkg/logging"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
	"github.com/google/uuid"
)

var (
	_ monitoredresource.Interface = (*stackdriverMonitoredResource)(nil)
)

type stackdriverMonitoredResource struct {
	resource string
	labels   map[string]string
}

// NewStackdriverMonitoredResource returns a monitored resource with the
// required labels filled out. This needs to be the correct resource type so we
// can compared the default stackdriver metrics with the custom metrics we're
// generating.
func NewStackdriverMonitoredResource(ctx context.Context, c *StackdriverConfig) monitoredresource.Interface {
	logger := logging.FromContext(ctx).Named("stackdriver")

	resource := "generic_task"
	labels := make(map[string]string)

	// On GCP we can fill in some of the information for GCE and GKE.
	detected := gcp.Autodetect()
	providedLabels := make(map[string]string)
	providedResource := ""
	if detected != nil {
		providedResource, providedLabels = detected.MonitoredResource()
		logger.Debugw("detected resource", "resource", providedResource, "labels", providedLabels)
	}

	if _, ok := providedLabels["project_id"]; !ok {
		labels["project_id"] = c.ProjectID
	} else {
		labels["project_id"] = providedLabels["project_id"]
	}

	if c.Service != "" {
		labels["job"] = c.Service
	} else {
		labels["job"] = "unknown"
	}

	// Transform "instance_id" to "task_id" or generate task_id
	if iid, ok := providedLabels["instance_id"]; ok {
		labels["task_id"] = iid
	}

	// Try to get task_id from metadata server.
	//
	// NOTE: This is essentially the same thing as gcp.Autodetect(). We're doing
	// this here in case something weird is happening in the autodetect.
	if labels["task_id"] == "" {
		iid, err := metadata.InstanceID()
		if err == nil {
			labels["task_id"] = iid
		}
	}

	// Worse case task_id
	if labels["task_id"] == "" {
		labels["task_id"] = uuid.New().String()
	}

	if zone, ok := providedLabels["zone"]; ok {
		labels["location"] = zone
	} else if loc, ok := providedLabels["location"]; ok {
		labels["location"] = loc
	} else {
		labels["location"] = "unknown"
		if c.LocationOverride != "" {
			labels["location"] = c.LocationOverride
		}
	}

	labels["namespace"] = c.Namespace

	filteredLabels := removeUnusedLabels(resource, labels)

	logger.Debugw("resource type defined", "resource", resource, "labels", labels, "filteredLabels", filteredLabels)
	return &stackdriverMonitoredResource{
		resource: resource,
		labels:   filteredLabels,
	}
}

func (s *stackdriverMonitoredResource) MonitoredResource() (string, map[string]string) {
	return s.resource, s.labels
}

// removeUnusedLabels deletes unused labels to not flood stackdriver.
func removeUnusedLabels(resource string, in map[string]string) map[string]string {
	// The labels each resource type requires.
	requiredLabels = map[string]map[string]bool{
		// https://cloud.google.com/monitoring/api/resources#tag_generic_task
		"generic_task": {"project_id": true, "location": true, "namespace": true, "job": true, "task_id": true},
	}

	ret := map[string]string{}
	for k, v := range in {
		if requiredLabels[k] {
			ret[k] = v
		}
	}

	return ret
}
