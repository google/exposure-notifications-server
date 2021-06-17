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

variable "project" {
  type        = string
  description = "GCP project for key server. Required."
}

variable "exposure_hosts" {
  type        = list(string)
  description = "List of domains upon which the exposure uploads are served."
  default     = []
}

variable "alert-notification-channel-paging" {
  type = map(any)
  default = {
    email = {
      labels = {
        email_address = "nobody@example.com"
      }
    }
    slack = {
      labels = {
        channel_name = "#paging-channel"
        auth_token   = "abr"
      }
    }
  }
  description = "Paging notification channels"
}

variable "alert-notification-channel-non-paging" {
  type = map(any)
  default = {
    email = {
      labels = {
        email_address = "nobody@example.com"
      }
    }
    slack = {
      labels = {
        channel_name = "#non-paging-channel"
        auth_token   = "non-paging channel"
      }
    }
  }
  description = "Non-paging notification channels"
}

variable "alert_on_human_accessed_secret" {
  type    = bool
  default = true

  description = "Alert when a human accesses a secret. You must enable DATA_READ audit logs for Secret Manager."
}

variable "alert_on_human_decrypted_value" {
  type    = bool
  default = true

  description = "Alert when a human accesses a secret. You must enable DATA_READ audit logs for Cloud KMS."
}

variable "alert_on_cloud_run_breakglass" {
  type    = bool
  default = true

  description = "Alert when a service is deployed that bypassed Binary Authorization."
}

variable "capture_export_file_downloads" {
  type    = bool
  default = true

  description = "Capture metrics about mobile devices downloading export files. This can be used to create alerts when values drop below acceptable thresholds."
}

variable "forward_progress_indicators" {
  type = map(object({
    metric = string
    window = number
  }))

  description = "Map of overrides for forward progress indicators. These are merged with the default variables. The window must be in seconds."
  default     = {}
}

terraform {
  required_version = ">= 0.14.2"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.51"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.51"
    }
  }
}
