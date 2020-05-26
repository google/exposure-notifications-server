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

# The default region for resources in the project, individual resources should
# have more specific variables defined to specify their regions to increase the
# flexibility of deployments
variable "region" {
  type    = string
  default = "us-central1"
}

# The region in which to put the SQL DB - it is currently configured to use
# PostgreSQL:
# https://cloud.google.com/sql/docs/postgres/locations
variable "db_region" {
  type = string
  default = "us-central1"
}

# The region in which to put the key management service:
# https://cloud.google.com/kms/docs/locations
variable "kms_location" {
  type = string
  default = "us-central1"
}

# The location for the app engine; this includes scheduler jobs for the app
# engine as defined by the appengine_region local, derived from this variable:
# https://cloud.google.com/appengine/docs/locations
variable "appengine_location" {
  type    = string
  default = "us-central"
}

# The appengine_region MUST use the same region as appengine_location but must
# also include the region number which must sometimes be omitted from
# appengine_location (as in the default values)
variable "appengine_region" {
  type    = string
  default = "us-central1"
}


# The region in which for cloudrun jobs are executed:
# https://cloud.google.com/run/docs/locations
variable "cloudrun_location" {
  type    = string
  default = "us-central1"
}

# The location holding the storage bucket for exported files:
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

terraform {
  required_providers {
    google      = "~> 3.20"
    google-beta = "~> 3.20"
    null        = "~> 2.1"
    random      = "~> 2.2"
  }
}
