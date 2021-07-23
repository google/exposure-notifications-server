# Copyright 2020 the Exposure Notifications Server authors
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
  playbook_prefix = "https://github.com/google/exposure-notifications-server/blob/main/docs/playbooks/alerts"
  custom_prefix   = "custom.googleapis.com/opencensus/en-server"

  second = 1
  minute = 60 * local.second
  hour   = 60 * local.minute

  forward_progress_indicators = merge(
    {
      # backup runs every 4h, alert after 2 failures
      "backup" = { metric = "backup/success", window = 8 * local.hour + 10 * local.minute },

      # cleanup-export runs every 4h, alert after 2 failures
      "cleanup-export" = { metric = "cleanup/export/success", window = 8 * local.hour + 10 * local.minute },

      # cleanup-exposure runs every 4h, alert after 2 failures
      "cleanup-exposure" = { metric = "cleanup/exposure/success", window = 8 * local.hour + 10 * local.minute },

      # export-batcher runs every 5m, alert after 3 failures
      "export-batcher" = { metric = "export/batcher/success", window = 15 * local.minute + 3 * local.minute },

      # export-worker runs every 1m but can take up to 5m to finish, alert
      # after ~2 failures
      "export-worker" = { metric = "export/worker/success", window = 10 * local.minute + 2 * local.minute },

      # export-importer-schedule runs every 15m, alert after 2 failures
      "export-importer-schedule" = { metric = "export-importer/schedule/success", window = 30 * local.minute + 5 * local.minute },

      # export-importer-import runs every 5m, alert after 3 failures
      "export-importer-import" = { metric = "export-importer/import/success", window = 15 * local.minute + 3 * local.minute },

      # jwks runs every 2m, alert after ~15 failures
      "jwks" = { metric = "jwks/success", window = 30 * local.minute + 5 * local.minute },

      # key-rotation runs every 4h, alert after 2 failures
      "key-rotation" = { metric = "key-rotation/success", window = 8 * local.hour + 10 * local.minute + 2 * local.minute },

      # mirror runs every 5m but has a default lock time of 15m, alert after 2 failures
      "mirror" = { metric = "mirror/success", window = 30 * local.minute + 5 * local.minute },
    },
    var.forward_progress_indicators,
  )
}

# This resource creates two conditions for each metric: one if the metric's
# threshold is <= 0 for the duration, and another of the metric is missing for
# the duration. This handles both the case when a job has never run and when a
# job previously ran but is now failing.
resource "google_monitoring_alert_policy" "ForwardProgress" {
  for_each = local.forward_progress_indicators

  project      = var.project
  display_name = "ForwardProgress-${each.key}"
  combiner     = "OR"

  conditions {
    display_name = "${each.key} failing"

    condition_threshold {
      filter   = "metric.type = \"${local.custom_prefix}/${each.value.metric}\" AND resource.type = \"generic_task\""
      duration = "${each.value.window}s"

      comparison      = "COMPARISON_LT"
      threshold_value = 1

      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_DELTA"
        group_by_fields      = ["resource.labels.job"]
        cross_series_reducer = "REDUCE_SUM"
      }

      trigger {
        count = 1
      }
    }
  }

  conditions {
    display_name = "${each.key} missing"

    condition_absent {
      filter   = "metric.type = \"${local.custom_prefix}/${each.value.metric}\" AND resource.type = \"generic_task\""
      duration = "${each.value.window}s"

      aggregations {
        alignment_period     = "60s"
        per_series_aligner   = "ALIGN_DELTA"
        group_by_fields      = ["resource.labels.job"]
        cross_series_reducer = "REDUCE_SUM"
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/ForwardProgressFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "probers" {
  project = var.project

  display_name = "HostDown"
  combiner     = "OR"
  conditions {
    display_name = "Host is unreachable"
    condition_monitoring_query_language {
      duration = "60s"
      query    = <<-EOT
      fetch
      uptime_url :: monitoring.googleapis.com/uptime_check/check_passed
      | align next_older(1m)
      | every 1m
      | group_by [resource.host], [val: fraction_true(value.check_passed)]
      | condition val < 20 '%'
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/HostDown.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_logging_metric" "stackdriver_export_error_count" {
  project     = var.project
  name        = "stackdriver_export_error_count"
  description = "Error occurred trying to export metrics to stackdriver"

  filter = <<-EOT
  resource.type="cloud_run_revision"
  jsonPayload.logger="stackdriver"
  jsonPayload.message="failed to export metric"
  EOT

  metric_descriptor {
    metric_kind = "DELTA"
    unit        = "1"
    value_type  = "INT64"
  }
}

resource "google_monitoring_alert_policy" "StackdriverExportFailed" {
  project      = var.project
  display_name = "StackdriverExportFailed"
  combiner     = "OR"
  conditions {
    display_name = "Stackdriver metric export error rate"
    condition_monitoring_query_language {
      duration = "900s"
      # NOTE: this query calculates the rate over a 5min window instead of
      # usual 1min window. This is intentional:
      # The rate window should be larger than the interval of the errors.
      # Currently we export to stackdriver every 2min, meaning if the export is
      # constantly failing, our calculated error rate with 1min window will
      # have the number oscillating between 0 and 1, and we would never get an
      # alert beacuase each time the value reaches 0 the timer to trigger the
      # alert is reset.
      #
      # Changing this to 5min window means the condition is "on" as soon as
      # there's a single export error and last at least 5min. The alert is
      # firing if the condition is "on" for >15min.
      query = <<-EOT
      fetch
      cloud_run_revision::logging.googleapis.com/user/stackdriver_export_error_count
      | align rate(5m)
      | group_by [resource.service_name], [val: sum(value.stackdriver_export_error_count)]
      | condition val > 0
      EOT
      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/StackdriverExportFailed.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
    google_logging_metric.stackdriver_export_error_count
  ]
}

resource "google_logging_metric" "human_accessed_secret" {
  name    = "human_accessed_secret"
  project = var.project
  filter  = <<EOT
protoPayload.@type="type.googleapis.com/google.cloud.audit.AuditLog"
protoPayload.serviceName="secretmanager.googleapis.com"
protoPayload.methodName=~"AccessSecretVersion$"
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
protoPayload.@type="type.googleapis.com/google.cloud.audit.AuditLog"
protoPayload.serviceName="cloudkms.googleapis.com"
protoPayload.methodName="Decrypt"
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
      duration = "0s"

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

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]
}

resource "google_monitoring_alert_policy" "HumanDecryptedValue" {
  count = var.alert_on_human_decrypted_value ? 1 : 0

  project      = var.project
  display_name = "HumanDecryptedValue"
  combiner     = "OR"

  conditions {
    display_name = "A non-service account decrypted something."

    condition_monitoring_query_language {
      duration = "0s"

      query = <<-EOT
      fetch global
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

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]
}

resource "google_logging_metric" "export_file_downloaded" {
  count = var.capture_export_file_downloads ? 1 : 0

  name        = "export_file_downloaded"
  description = "Incremented on each export file download."
  project     = var.project
  filter      = <<EOT
resource.type="http_load_balancer"
httpRequest.requestUrl=~"/index.txt$"
httpRequest.status=200
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"

    labels {
      key         = "path"
      value_type  = "STRING"
      description = "Path of the export"
    }

    labels {
      key         = "platform"
      value_type  = "STRING"
      description = "Mobile operating system"
    }
  }

  label_extractors = {
    "path"     = "REGEXP_EXTRACT(httpRequest.requestUrl, \"https?://.+/(.+)/index\\\\.txt\")"
    "platform" = "REGEXP_EXTRACT(httpRequest.userAgent, \"(Android|Darwin)\")"
  }
}

resource "google_logging_metric" "export_archive_downloaded" {
  count = var.capture_export_file_downloads ? 1 : 0

  name        = "export_archive_downloaded"
  description = "Incremented on each export zip file download."
  project     = var.project
  filter      = <<EOT
resource.type="http_load_balancer"
httpRequest.requestUrl=~".zip$"
httpRequest.status=200
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"

    labels {
      key         = "path"
      value_type  = "STRING"
      description = "Path of the export"
    }

    labels {
      key         = "platform"
      value_type  = "STRING"
      description = "Mobile operating system"
    }
  }

  label_extractors = {
    "path"     = "REGEXP_EXTRACT(httpRequest.requestUrl, \"https?://.+/(.+/.+)\\\\.zip\")"
    "platform" = "REGEXP_EXTRACT(httpRequest.userAgent, \"(Android|Darwin)\")"
  }
}

resource "google_logging_metric" "cloud_run_breakglass" {
  name    = "cloud_run_breakglass"
  project = var.project

  filter = <<EOT
protoPayload.@type="type.googleapis.com/google.cloud.audit.AuditLog"
protoPayload.serviceName="run.googleapis.com"
protoPayload.status.message:"breakglass"
resource.labels.revision_name!=""
EOT

  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"

    labels {
      key         = "revision"
      value_type  = "STRING"
      description = "Name of the revision which was deployed with breakglass"
    }
  }

  label_extractors = {
    "revision" = "EXTRACT(resource.labels.revision_name)"
  }
}

resource "google_monitoring_alert_policy" "CloudRunBreakglass" {
  count = var.alert_on_cloud_run_breakglass ? 1 : 0

  project      = var.project
  display_name = "CloudRunBreakglass"
  combiner     = "OR"

  conditions {
    display_name = "A service was deployed that bypassed Binary Authorization"

    condition_monitoring_query_language {
      duration = "0s"

      query = <<-EOT
      fetch global
      | metric 'logging.googleapis.com/user/${google_logging_metric.cloud_run_breakglass.name}'
      | align rate(5m)
      | every 1m
      | group_by [resource.project_id],
          [val: aggregate(value.cloud_run_breakglass)]
      | condition val > 0
      EOT

      trigger {
        count = 1
      }
    }
  }

  documentation {
    content   = "${local.playbook_prefix}/CloudRunBreakglass.md"
    mime_type = "text/markdown"
  }

  notification_channels = [for x in values(google_monitoring_notification_channel.paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}
