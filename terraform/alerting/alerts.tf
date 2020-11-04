# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

locals {
  slow_services = format("(%s)", join("|", [
    "export",
  ]))
}

resource "google_monitoring_alert_policy" "LatencyTooHigh" {
  combiner     = "OR"
  display_name = "LatencyTooHigh"
  conditions {
    display_name = "p50 request latency"
    condition_monitoring_query_language {
      duration = "180s"
      # NOTE: this is a bit complex because we want to have a single latency
      # alert (not two alerts one for slow and one for normal), and we can only
      # have one mql query in a conditions block.
      query = <<-EOT
      fetch
      cloud_run_revision :: run.googleapis.com/request_latencies
      | add
      [type: if(resource.service_name =~ '${local.slow_services}', 'SLOW', 'NORMAL')]
      | align delta(1m)
      | every 1m
      | group_by [resource.service_name, type],
      [val: percentile(value.request_latencies, 50)]
      | condition
          (type == 'SLOW' && val > 20000 'ms')
        ||(type == 'NORMAL' && val > 10000 'ms')
      EOT
      trigger {
        count = 1
      }
    }
  }
  documentation {
    content   = <<-EOT
    Our latency is too high for one or more Cloud Run services!
    EOT
    mime_type = "text/markdown"
  }
}
