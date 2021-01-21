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

  user_project_override = true
}

provider "google-beta" {
  project = var.project
  region  = var.region

  user_project_override = true
}

# To generate passwords.
provider "random" {}

data "google_project" "project" {
  project_id = var.project
}

# Cloud Resource Manager needs to be enabled first, before other services.
resource "google_project_service" "resourcemanager" {
  project            = var.project
  service            = "cloudresourcemanager.googleapis.com"
  disable_on_destroy = false
}

resource "google_project_service" "services" {
  project = data.google_project.project.project_id
  for_each = toset([
    "binaryauthorization.googleapis.com",
    "cloudbuild.googleapis.com",
    "cloudkms.googleapis.com",
    "cloudscheduler.googleapis.com",
    "compute.googleapis.com",
    "containeranalysis.googleapis.com",
    "containerregistry.googleapis.com",
    "iam.googleapis.com",
    "monitoring.googleapis.com",
    "redis.googleapis.com",
    "run.googleapis.com",
    "secretmanager.googleapis.com",
    "servicenetworking.googleapis.com",
    "sql-component.googleapis.com",
    "sqladmin.googleapis.com",
    "stackdriver.googleapis.com",
    "storage-api.googleapis.com",
    "vpcaccess.googleapis.com",
  ])
  service            = each.value
  disable_on_destroy = false

  depends_on = [
    google_project_service.resourcemanager,
  ]
}

resource "google_compute_global_address" "private_ip_address" {
  name          = "private-ip-address"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = "projects/${data.google_project.project.project_id}/global/networks/default"

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_service_networking_connection" "private_vpc_connection" {
  network                 = "projects/${data.google_project.project.project_id}/global/networks/default"
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
    google_project_service.services["servicenetworking.googleapis.com"],
  ]
}

resource "google_vpc_access_connector" "connector" {
  project        = data.google_project.project.project_id
  name           = "serverless-vpc-connector"
  region         = var.network_location
  network        = "default"
  ip_cidr_range  = "10.8.0.0/28"
  max_throughput = var.vpc_access_connector_max_throughput

  depends_on = [
    google_service_networking_connection.private_vpc_connection,
    google_project_service.services["compute.googleapis.com"],
    google_project_service.services["vpcaccess.googleapis.com"],
  ]
}

# Build creates the container images. It does not deploy or promote them.
resource "null_resource" "build" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID = data.google_project.project.project_id
      TAG        = "initial"

      BINAUTHZ_ATTESTOR    = google_binary_authorization_attestor.built-by-ci.id
      BINAUTHZ_KEY_VERSION = trimprefix(data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.id, "//cloudkms.googleapis.com/v1/")
    }

    command = "${path.module}/../scripts/build"
  }

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_storage_bucket_iam_member.cloudbuild-cache,
    google_binary_authorization_attestor_iam_member.ci-attestor,
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
  common_cloudrun_env_vars = {
    PROJECT_ID = var.project

    DB_HOST           = google_sql_database_instance.db-inst.private_ip_address
    DB_NAME           = google_sql_database.db.name
    DB_PASSWORD       = "secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
    DB_POOL_MAX_CONNS = "10"
    DB_POOL_MIN_CONNS = "2"
    DB_SSLCERT        = "secret://${google_secret_manager_secret_version.db-secret-version["sslcert"].id}?target=file"
    DB_SSLKEY         = "secret://${google_secret_manager_secret_version.db-secret-version["sslkey"].id}?target=file"
    DB_SSLMODE        = "verify-ca"
    DB_SSLROOTCERT    = "secret://${google_secret_manager_secret_version.db-secret-version["sslrootcert"].id}?target=file"
    DB_USER           = google_sql_user.user.name
  }
}

# Cloud Scheduler requires AppEngine projects!
resource "google_app_engine_application" "app" {
  project     = data.google_project.project.project_id
  location_id = var.appengine_location
}

# Create a helper for generating the local environment configuration - this is
# disabled by default because it includes sensitive information to the project.
resource "local_file" "env" {
  count = var.create_env_file == true ? 1 : 0

  filename        = "${path.root}/.env"
  file_permission = "0600"

  sensitive_content = <<EOF
export PROJECT_ID="${var.project}"
export REGION="${var.region}"

# Note: these configurations assume you're using the Cloud SQL proxy!
export DB_CONN="${google_sql_database_instance.db-inst.connection_name}"
export DB_DEBUG="true"
export DB_HOST="127.0.0.1"
export DB_NAME="${google_sql_database.db.name}"
export DB_PASSWORD="secret://${google_secret_manager_secret_version.db-secret-version["password"].id}"
export DB_PORT="5432"
export DB_SSLMODE="disable"
export DB_USER="${google_sql_user.user.name}"

export BINAUTHZ_ATTESTOR="${google_binary_authorization_attestor.built-by-ci.id}"
export BINAUTHZ_KEY_VERSION="${trimprefix(data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.id, "//cloudkms.googleapis.com/v1/")}"
EOF
}

output "project_id" {
  value = data.google_project.project.project_id
}

output "project_number" {
  value = data.google_project.project.number
}

output "region" {
  value = var.region
}

output "db_location" {
  value = var.db_location
}

output "network_location" {
  value = var.network_location
}

output "kms_location" {
  value = var.kms_location
}

output "appengine_location" {
  value = var.appengine_location
}

output "cloudscheduler_location" {
  value = var.cloudscheduler_location
}

output "cloudrun_location" {
  value = var.cloudrun_location
}

output "storage_location" {
  value = var.storage_location
}
