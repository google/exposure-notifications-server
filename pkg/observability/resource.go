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
	"encoding/base64"

	"github.com/google/exposure-notifications-server/pkg/logging"

	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
	"github.com/google/uuid"
)

var (
	_ monitoredresource.Interface = (*stackdriverMonitoredResource)(nil)

	// The labels each resource type requires.
	requiredLabels = map[string]map[string]bool{
		// https://cloud.google.com/monitoring/api/resources#tag_generic_task
		"generic_task": map[string]bool{"project_id": true, "location": true, "namespace": true, "job": true, "task_id": true},
		// https://cloud.google.com/monitoring/api/resources#tag_gke_container
		"gke_container": map[string]bool{"project_id": true, "cluster_name": true, "namespace_id": true, "instance_id": true, "pod_id": true, "container_name": true, "zone": true},
		// https://cloud.google.com/monitoring/api/resources#tag_cloud_run_revision
		"cloud_run_revision": map[string]bool{"project_id": true, "service_name": true, "revision_name": true, "location": true, "configuration_name": true},
	}
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
	if detected != nil {
		resource, providedLabels = detected.MonitoredResource()
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
	} else {
		labels["task_id"] = base64.StdEncoding.EncodeToString(uuid.NodeID())
	}

	if zone, ok := providedLabels["zone"]; ok {
		labels["location"] = zone
	} else if loc, ok := providedLabels["location"]; ok {
		labels["location"] = loc
	} else {
		labels["location"] = "unknown"
	}

	labels["namespace"] = c.Namespace

	// Are on Cloud Run Managed?
	//
	// https://cloud.google.com/monitoring/api/resources#tag_cloud_run_revision
	// https://cloud.google.com/run/docs/reference/container-contract#env-vars
	if c.Service != "" && c.Revision != "" {
		resource = "cloud_run_revision"
		labels["service_name"] = c.Service
		labels["revision_name"] = c.Revision
		labels["configuration_name"] = c.Namespace
	}

	if _, ok := requiredLabels[resource]; !ok {
		logger.Warnw("unknown resource type", "resource", resource, "labels", labels)
	}

	// Delete unused labels to not flood stackdriver.
	for k := range labels {
		if _, ok := requiredLabels[k]; !ok {
			delete(labels, k)
		}
	}

	return &stackdriverMonitoredResource{
		resource: resource,
		labels:   labels,
	}
}

func (s *stackdriverMonitoredResource) MonitoredResource() (string, map[string]string) {
	return s.resource, s.labels
}
