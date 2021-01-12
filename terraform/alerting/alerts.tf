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
  p50_latency_thresholds = {
    export         = "10min"
    cleanup-export = "10min"
    generate       = "2min"
  }
  p50_latency_thresholds_default = "10s"

  p50_latency_condition = join("\n  || ", concat(
    [
      for k, v in local.p50_latency_thresholds :
      "(resource.service_name == '${k}' && val > ${replace(v, "/(\\d+)(.*)/", "$1 '$2'")})"
    ],
    [
      "(val > ${replace(local.p50_latency_thresholds_default, "/(\\d+)(.*)/", "$1 '$2'")})"
    ]
  ))
}

locals {
  playbook_prefix = "https://github.com/google/exposure-notifications-server/blob/main/docs/playbooks/alerts"
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
    content   = "${local.playbook_prefix}/LatencyTooHigh.md"
    mime_type = "text/markdown"
  }
  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]
  depends_on = [
    google_monitoring_notification_channel.channels
  ]
}

resource "google_monitoring_alert_policy" "CloudSchedulerJobFailed" {
  project      = var.project
  display_name = "CloudSchedulerJobFailed"
  combiner     = "OR"
  conditions {
    display_name = "Cloud Scheduler Job Error Ratio"
    condition_monitoring_query_language {
      duration = "900s"
      # Uses rate(5m). See the reasoning above.
      query = <<-EOT
      fetch cloud_scheduler_job::logging.googleapis.com/log_entry_count
      | filter (metric.severity == 'ERROR')
      | align rate(5m)
      | group_by [resource.job_id], [val: aggregate(value.log_entry_count)]
      | condition val > 0
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/CloudSchedulerJobFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]

  depends_on = [
    google_monitoring_notification_channel.channels
  ]
}

resource "google_logging_metric" "human_accessed_secret" {
  name    = "human_accessed_secret"
  project = var.project
  filter  = <<EOT
resource.type="audited_resource"
resource.labels.service="secretmanager.googleapis.com"
resource.labels.method:"AccessSecretVersion"
protoPayload.authenticationInfo.principalEmail!~"gserviceaccount.com$"
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"

    labels {
      key         = "email"
      value_type  = "STRING"
      description = "Email address of the violating principal."
    }
    labels {
      key         = "secret"
      value_type  = "STRING"
      description = "Full resource ID of the secret."
    }
  }
  label_extractors = {
    "email"  = "EXTRACT(protoPayload.authenticationInfo.principalEmail)"
    "secret" = "EXTRACT(protoPayload.resourceName)"
  }
}
resource "google_logging_metric" "human_decrypted_value" {
  name    = "human_decrypted_value"
  project = var.project
  filter  = <<EOT
resource.type="audited_resource"
resource.labels.service="cloudkms.googleapis.com"
resource.labels.method:"Decrypt"
protoPayload.authenticationInfo.principalEmail!~"gserviceaccount.com$"
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"

    labels {
      key         = "email"
      value_type  = "STRING"
      description = "Email address of the violating principal."
    }
    labels {
      key         = "key"
      value_type  = "STRING"
      description = "Full resource ID of the key."
    }
  }
  label_extractors = {
    "email" = "EXTRACT(protoPayload.authenticationInfo.principalEmail)"
    "key"   = "EXTRACT(protoPayload.resourceName)"
  }
}

resource "google_monitoring_alert_policy" "HumanAccessedSecret" {
  count = var.alert_on_human_accessed_secret ? 1 : 0

  project      = var.project
  display_name = "HumanAccessedSecret"
  combiner     = "OR"

  conditions {
    display_name = "A non-service account accessed a secret."

    condition_monitoring_query_language {
      duration = "60s"

      query = <<-EOT
      fetch audited_resource
      | metric 'logging.googleapis.com/user/${google_logging_metric.human_accessed_secret.name}'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [val: aggregate(value.human_accessed_secret)]
      | condition val > 0
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HumanAccessedSecret.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]
}

resource "google_monitoring_alert_policy" "HumanDecryptedValue" {
  count = var.alert_on_human_decrypted_value ? 1 : 0

  project      = var.project
  display_name = "HumanDecryptedValue"
  combiner     = "OR"

  conditions {
    display_name = "A non-service account decrypted something."

    condition_monitoring_query_language {
      duration = "60s"

      query = <<-EOT
      fetch audited_resource
      | metric 'logging.googleapis.com/user/${google_logging_metric.human_decrypted_value.name}'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [val: aggregate(value.human_decrypted_value)]
      | condition val > 0
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HumanDecryptedValue.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.channels) : x.id]
}
