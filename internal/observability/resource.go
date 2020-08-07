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
	"encoding/base64"

	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource/gcp"
	"github.com/google/uuid"
)

var _ monitoredresource.Interface = (*StackdriverMonitoredResoruce)(nil)

type StackdriverMonitoredResoruce struct {
	resource string
	labels   map[string]string
}

func NewStackdriverMonitoredResoruce(c *StackdriverConfig) monitoredresource.Interface {
	resource := "generic_task"
	labels := make(map[string]string)

	// On GCP we can fill in some of the information.
	detected := gcp.Autodetect()
	providedLabels := make(map[string]string)
	if detected != nil {
		_, providedLabels = detected.MonitoredResource()
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

	return &StackdriverMonitoredResoruce{
		resource: resource,
		labels:   labels,
	}
}

func (s *StackdriverMonitoredResoruce) MonitoredResource() (string, map[string]string) {
	return s.resource, s.labels
}
