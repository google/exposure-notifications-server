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

const ResourceType = "enservice"

var _ monitoredresource.Interface = (*StackdriverMonitoredResoruce)(nil)

var (
	nodeUUID string
)

func init() {
	nodeUUID = base64.StdEncoding.EncodeToString(uuid.NodeID())
}

type StackdriverMonitoredResoruce struct {
	resource string
	labels   map[string]string
}

func NewStackdriverMonitoredResoruce(c *StackdriverConfig) monitoredresource.Interface {
	detected := gcp.Autodetect()
	resource := ResourceType
	labels := make(map[string]string)
	if detected != nil {
		resource, labels = detected.MonitoredResource()
	}

	/*
		l := make(map[string]string)
		l["project_id"] = c.ProjectID
		if c.Revision != "" {
			l["revision"] = c.Revision
		}
		if c.Service != "" {
			l["service"] = c.Service
		}
	*/

	return &StackdriverMonitoredResoruce{
		resource: resource,
		labels:   labels,
	}
}

func (s *StackdriverMonitoredResoruce) MonitoredResource() (string, map[string]string) {
	return s.resource, s.labels
}
