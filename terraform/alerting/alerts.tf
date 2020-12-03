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
  p50_latency_thresholds_in_seconds = {
    export         = 600
    cleanup-export = 20
    generate       = 30
  }
  p50_latency_thresholds_in_seconds_default = 10

  p50_latency_condition = join("\n|| ", concat(
    [
      for k, v in local.p50_latency_thresholds_in_seconds :
      "(resource.service_name == '${k}' && val > ${v * 1000} 'ms')"
    ],
    [
      "(val > ${local.p50_latency_thresholds_in_seconds_default * 1000} 'ms')"
    ]
  ))
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
      | align delta(1m)
      | every 1m
      | group_by [resource.service_name],
      [val: percentile(value.request_latencies, 50)]
      | condition ${local.p50_latency_condition}
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
  notification_channels = [
    google_monitoring_notification_channel.email.id
  ]
}
