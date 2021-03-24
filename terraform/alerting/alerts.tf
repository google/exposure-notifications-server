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
  playbook_prefix = "https://github.com/google/exposure-notifications-server/blob/main/docs/playbooks/alerts"
  custom_prefix   = "custom.googleapis.com/opencensus/en-server"

  forward_progress_indicators = merge(
    {
      // backup runs every 4h, alert after 2 failures
      "backup" = { metric = "backup/success", window = "485m" },

      // cleanup-export runs every 4h, alert after 2 failures
      "cleanup-export" = { metric = "cleanup/export/success", window = "485m" },

      // cleanup-exposure runs every 4h, alert after 2 failures
      "cleanup-exposure" = { metric = "cleanup/exposure/success", window = "485m" },

      // export-importer-schedule runs every 15m, alert after 2 failures
      "export-importer-schedule" = { metric = "export-importer/schedule/success", window = "35m" },

      // export-importer-import runs every 5m, alert after 3 failures
      "export-importer-import" = { metric = "export-importer/import/success", window = "20m" },

      // jwks runs every 2m, alert after ~15 failures
      "jwks" = { metric = "jwks/success", window = "30m" },

      // key-rotation runs every 4h, alert after 2 failures
      "key-rotation" = { metric = "key-rotation/success", window = "485m" },

      // mirror runs every 5m but has a default lock time of 15m, alert after 2 failures
      "mirror" = { metric = "mirror/success", window = "35m" },
    },
    var.forward_progress_indicators,
  )
}

resource "google_monitoring_alert_policy" "CloudSchedulerJobFailed" {
  project      = var.project
  display_name = "CloudSchedulerJobFailed"
  combiner     = "OR"
  conditions {
    display_name = "Cloud Scheduler Job Error Ratio"
    condition_monitoring_query_language {
      duration = "0s"
      query    = <<-EOT
      # NOTE: The query below will be evaluated every 30s. It will look at the latest point that
      # represents the total count of log entries for the past 10m (align delta (10m)),
      # and fork it to two streams, one representing only ERROR logs, one representing ALL logs,
      # and do an outer join with default value 0 for the first stream.
      # Then it computes the first stream / second stream getting the ratio of ERROR logs over ALL logs,
      # and finally group by. The alert will fire when the error rate was 100% for the last 10 mins.
      fetch cloud_scheduler_job
      | metric 'logging.googleapis.com/log_entry_count'
      | align delta(10m)
      | { t_0: filter metric.severity == 'ERROR'
        ; t_1: ident }
      | outer_join [0]
      | value
          [t_0_value_log_entry_count_div:
            div(t_0.value.log_entry_count, t_1.value.log_entry_count)]
      | group_by [resource.job_id],
          [t_0_value_log_entry_count_div_sum: sum(t_0_value_log_entry_count_div)]
      | condition t_0_value_log_entry_count_div_sum >= 1
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

  notification_channels = [for x in values(google_monitoring_notification_channel.non-paging) : x.id]

  depends_on = [
    null_resource.manual-step-to-enable-workspace,
  ]
}

resource "google_monitoring_alert_policy" "ForwardProgressFailed" {
  for_each = local.forward_progress_indicators

  project      = var.project
  display_name = "ForwardProgressFailed-${each.key}"
  combiner     = "OR"

  conditions {
    display_name = each.key

    condition_monitoring_query_language {
      duration = "0s"
      query    = <<-EOT
      fetch generic_task
      | metric '${local.custom_prefix}/${each.value.metric}'
      | align delta(1m)
      | group_by [], [val: aggregate(value.success)]
      | absent_for ${each.value.window}
      EOT

      trigger {
        count = 1
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
    "path"     = "REGEXP_EXTRACT(httpRequest.requestUrl, \"https?://.+/(.+)/index\\\\\\\\.txt\")"
    "platform" = "REGEXP_EXTRACT(httpRequest.userAgent, \"(Android|Darwin)\")"
  }
}
