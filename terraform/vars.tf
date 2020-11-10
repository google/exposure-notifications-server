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

variable "create_env_file" {
  type    = bool
  default = false

  description = "Create a .env file in the module directory with variables set to the configuration values."
}

# The default region for resources in the project, individual resources should
# have more specific variables defined to specify their region/location which
# increases the flexibility of deployments
variable "region" {
  type    = string
  default = "us-central1"
}

# The region in which to put the SQL DB: it is currently configured to use
# PostgreSQL.
# https://cloud.google.com/sql/docs/postgres/locations
variable "db_location" {
  type    = string
  default = "us-central1"
}

# The name of the database.
variable "db_name" {
  type    = string
  default = "en-server"
}

# The region for the networking components.
# https://cloud.google.com/compute/docs/regions-zones
variable "network_location" {
  type    = string
  default = "us-central1"
}

# The region for the key management service.
# https://cloud.google.com/kms/docs/locations
variable "kms_location" {
  type    = string
  default = "us-central1"
}

# Name of the key ring for export signing keys.
variable "kms_export_signing_key_ring_name" {
  type    = string
  default = "export-signing"
}

# Name of the key ring for revision tokens.
variable "kms_revision_tokens_key_ring_name" {
  type    = string
  default = "revision-tokens"
}

# The location for the app engine; this implicitly defines the region for
# scheduler jobs as specified by the cloudscheduler_location variable but the
# values are sometimes different (as in the default values) so they are kept as
# separate variables.
# https://cloud.google.com/appengine/docs/locations
variable "appengine_location" {
  type    = string
  default = "us-central"
}

# The cloudscheduler_location MUST use the same region as appengine_location but
# it must include the region number even if this is omitted from the
# appengine_location (as in the default values).
variable "cloudscheduler_location" {
  type    = string
  default = "us-central1"
}


# The region in which cloudrun jobs are executed.
# https://cloud.google.com/run/docs/locations
variable "cloudrun_location" {
  type    = string
  default = "us-central1"
}

# The location holding the storage bucket for exported files.
# https://cloud.google.com/storage/docs/locations
variable "storage_location" {
  type    = string
  default = "US"
}

variable "project" {
  type = string
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

variable "cloudsql_max_connections" {
  type    = number
  default = 100000

  description = "Maximum number of allowed connections. If you change to a smaller instance size, you must lower this number."
}

variable "cloudsql_backup_location" {
  type    = string
  default = "us"

  description = "Location in which to backup the database."
}

variable "export_worker_cron_schedule" {
  type    = string
  default = "* * * * *"

  description = "Schedule to execute the export worker service."
}

variable "export_create_batches_cron_schedule" {
  type    = string
  default = "*/5 * * * *"

  description = "Schedule to execute the export create batches service."
}

variable "cleanup_exposure_worker_cron_schedule" {
  type    = string
  default = "0 */4 * * *"

  description = "Schedule to execute the cleanup exposure worker service."
}

variable "cleanup_export_worker_cron_schedule" {
  type    = string
  default = "0 */6 * * *"

  description = "Schedule to execute the cleanup export worker service."
}

variable "generate_cron_schedule" {
  type    = string
  default = "0 0 1 1 0"

  description = "Schedule to execute the generation service."
}

variable "generate_regions" {
  type    = list(string)
  default = []

  description = "List of regions for which to generate data."
}

variable "enable_cdn_for_exports" {
  type    = bool
  default = false

  description = "Enable Cloud CDN on the export bucket."
}

variable "debugger_invokers" {
  type    = list(string)
  default = []

  description = "List of IAM entities that can invoke the debugger. This should be of the form user:[email], serviceAccount:[email], or group:[email]."
}

variable "service_environment" {
  type    = map(map(string))
  default = {}

  description = "Per-service environment overrides."
}

variable "debugger_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the debugger is served."
}

variable "export_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which exports should be served."
}

variable "exposure_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the exposure uploads are served."
}

variable "federationout_hosts" {
  type    = list(string)
  default = []

  description = "List of domains upon which the federationout service is served."
}

variable "vpc_access_connector_max_throughput" {
  type    = number
  default = 1000

  description = "Maximum provisioned traffic throughput in Mbps"
}

terraform {
  required_version = ">= 0.13"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.36"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.36"
    }
    local = {
      source  = "hashicorp/local"
      version = "~> 2.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 2.1"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 2.3"
    }
  }
}
