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

variable "region" {
  type    = string
  default = "us-central1"
}

variable "appengine_location" {
  type    = string
  default = "us-central"
}

variable "project" {
  type = string
}

variable "repo_owner" {
  type    = string
  default = "google"
}

variable "repo_name" {
  type    = string
  default = "exposure-notifications-server"
}

variable "cloudsql_tier" {
  type    = string
  default = "db-custom-8-30720"

  description = "Size of the Cloud SQL tier. Set to db-custom-1-3840 or a smaller instance for local dev."
}

variable "cloudsql_disk_size_gb" {
  type    = number
  default = 256

  description = "Size of the Cloud SQL disk, in GB."
}

variable "generate_cron_schedule" {
  type    = string
  default = "0 0 1 1 0"

  description = "Schedule to execute the generation service."
}

variable "registry_cleanup_cron_schedule" {
  type    = string
  default = "0 0 1 1 0"

  description = "Schedule to execute cleanup of old Container Registry images."
}

variable "service_environment" {
  type    = map(map(string))
  default = {}

  description = "Per-service environment overrides."
}

terraform {
  required_providers {
    google      = "~> 3.20"
    google-beta = "~> 3.20"
    null        = "~> 2.1"
    random      = "~> 2.2"
  }
}
