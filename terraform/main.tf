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

provider "google" {
  project = var.project
  region  = var.region
}

# For beta-only resources like secrets-manager
provider "google-beta" {
  project = var.project
  region  = var.region
}

# To generate passwords.
provider "random" {}

data "google_project" "project" {
  project_id = var.project
}

data "google_compute_network" "default" {
  name = "default"
}

resource "google_project_service" "services" {
  project = data.google_project.project.project_id
  for_each = toset([
    "cloudbuild.googleapis.com",
    "cloudkms.googleapis.com",
    "cloudscheduler.googleapis.com",
    "compute.googleapis.com",
    "containerregistry.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "servicenetworking.googleapis.com",
    "sql-component.googleapis.com",
    "sqladmin.googleapis.com",
    "storage-api.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false
}

resource "google_compute_global_address" "private_ip_address" {
  name          = "private-ip-address"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = data.google_compute_network.default.self_link

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = data.google_compute_network.default.self_link
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]
}

# Build creates the container images. It does not deploy or promote them.
resource "null_resource" "build" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID = data.google_project.project.project_id
      REGION     = var.region
      SERVICES   = "all"
      TAG        = "initial"
    }

    command = "${path.module}/../scripts/build"
  }

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

# Grant Cloud Build the ability to deploy images. It does not do so in these
# configurations, but it will do future deployments.
resource "google_project_iam_member" "cloudbuild-deploy" {
  project = data.google_project.project.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

locals {
  common_cloudrun_env_vars = [
    {
      name  = "DB_POOL_MIN_CONNS"
      value = "2"
    },
    {
      name  = "DB_POOL_MAX_CONNS"
      value = "50"
    },
    {
      name  = "DB_SSLMODE"
      value = "verify-ca"
    },
    {
      name  = "DB_HOST"
      value = google_sql_database_instance.db-inst.private_ip_address
    },
    {
      name  = "DB_NAME"
      value = google_sql_database.db.name
    },
    {
      name  = "DB_SSLCERT"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslcert"].id}?target=file"
    },

    {
      name  = "DB_SSLKEY"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslkey"].id}?target=file"
    },

    {
      name  = "DB_SSLROOTCERT"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["sslrootcert"].id}?target=file"
    },

    {
      name  = "DB_USER"
      value = google_sql_user.user.name
    },

    {
      name  = "DB_PASSWORD"
      value = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
    },
  ]
}

# Cloud Scheduler requires AppEngine projects!
resource "google_app_engine_application" "app" {
  project     = data.google_project.project.project_id
  location_id = var.appengine_location
}

output "region" {
  value = var.region
}

output "project" {
  value = data.google_project.project.project_id
}
