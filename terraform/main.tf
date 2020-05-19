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

# This is to ensure that the project / gcloud auth are correctly configured.
resource "null_resource" "gcloud_check" {
  provisioner "local-exec" {
    command = "command -v gcloud &>/dev/null || (echo 'Please install and authenticate with gcloud!' && exit 127)"
  }
}

data "google_project" "project" {
  project_id = var.project
}

resource "google_project_service" "services" {
  project = data.google_project.project.project_id
  for_each = toset(["run.googleapis.com", "cloudkms.googleapis.com", "secretmanager.googleapis.com", "storage-api.googleapis.com", "cloudscheduler.googleapis.com",
  "sql-component.googleapis.com", "cloudbuild.googleapis.com", "servicenetworking.googleapis.com", "compute.googleapis.com", "sqladmin.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}

resource "random_string" "db-name" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_compute_global_address" "private_ip_address" {
  provider = google-beta

  name          = "private-ip-address"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = "default"

  depends_on = [google_project_service.services["compute.googleapis.com"]]
}

resource "google_service_networking_connection" "private_vpc_connection" {
  provider = google-beta

  network                 = "default"
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_address.name]
}

resource "google_sql_database_instance" "db-inst" {
  project          = data.google_project.project.project_id
  region           = var.region
  database_version = "POSTGRES_11"
  name             = "contact-tracing-${random_string.db-name.result}"
  settings {
    tier              = var.cloudsql_tier
    disk_size         = var.cloudsql_disk_size_gb
    availability_type = "REGIONAL"
    backup_configuration {
      enabled    = true
      start_time = "02:00"
    }
    maintenance_window {
      day          = 7
      hour         = 2
      update_track = "stable"
    }
    ip_configuration {
      require_ssl     = true
      private_network = "projects/${data.google_project.project.project_id}/global/networks/${google_service_networking_connection.private_vpc_connection.network}"
    }
  }
  lifecycle {
    prevent_destroy = true
  }
  depends_on = [google_project_service.services["sql-component.googleapis.com"]]
}

resource "google_sql_ssl_cert" "client_cert" {
  common_name = "apollo"
  instance    = google_sql_database_instance.db-inst.name
}

resource "google_secret_manager_secret" "ssl-ca-cert" {
  provider  = google-beta
  secret_id = "dbServerCA"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "db-ca-cert" {
  provider    = google-beta
  secret      = google_secret_manager_secret.ssl-ca-cert.id
  secret_data = google_sql_ssl_cert.client_cert.server_ca_cert
}

resource "google_secret_manager_secret" "ssl-key" {
  provider  = google-beta
  secret_id = "dbClientKey"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "db-key" {
  provider    = google-beta
  secret      = google_secret_manager_secret.ssl-key.id
  secret_data = google_sql_ssl_cert.client_cert.private_key
}

resource "google_secret_manager_secret" "ssl-cert" {
  provider  = google-beta
  secret_id = "dbClientCert"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "db-cert" {
  provider    = google-beta
  secret      = google_secret_manager_secret.ssl-cert.id
  secret_data = google_sql_ssl_cert.client_cert.cert
}

resource "random_password" "userpassword" {
  length  = 16
  special = false
}

resource "google_sql_user" "user" {
  instance = google_sql_database_instance.db-inst.name
  name     = "notification"
  password = random_password.userpassword.result
}

resource "google_sql_database" "db" {
  instance = google_sql_database_instance.db-inst.name

  name    = "main"
  project = data.google_project.project.project_id
}

resource "google_secret_manager_secret" "db-pwd" {
  provider  = google-beta
  secret_id = "dbPassword"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services["secretmanager.googleapis.com"]]
}

resource "google_project_iam_member" "cloudbuild-secrets" {
  project = data.google_project.project.project_id
  role    = "roles/secretmanager.secretAccessor"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [google_project_service.services["cloudbuild.googleapis.com"]]
}

resource "google_project_iam_member" "cloudbuild-sql" {
  project = data.google_project.project.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [google_project_service.services["cloudbuild.googleapis.com"]]
}

resource "google_cloudbuild_trigger" "update-schema" {
  provider = google-beta
  count    = var.use_build_triggers ? 1 : 0

  name        = "update-schema"
  description = "Build the containers for the schema migrator and run it to ensure the DB is up to date."
  filename    = "builders/schema.yaml"
  github {
    owner = var.repo_owner
    name  = var.repo_name
    push {
      branch = "^master$"
    }
  }
  substitutions = {
    "_CLOUDSQLPATH" : "${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
    "_PORT" : "5432"
    "_PASSWORD_SECRET" : google_secret_manager_secret.db-pwd.secret_id
    "_USER" : google_sql_user.user.name
    "_NAME" : google_sql_database.db.name
    "_SSLMODE" : "disable"
  }
  depends_on = [google_project_iam_member.cloudbuild-secrets, google_project_iam_member.cloudbuild-sql]
}

resource "null_resource" "submit-update-schema" {
  provisioner "local-exec" {
    command = "gcloud builds submit ../ --config ../builders/schema.yaml --project ${data.google_project.project.project_id} --substitutions=_PORT=5432,_PASSWORD_SECRET=${google_secret_manager_secret.db-pwd.secret_id},_USER=${google_sql_user.user.name},_NAME=${google_sql_database.db.name},_SSLMODE=disable,_CLOUDSQLPATH=${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
  }
  depends_on = [
    null_resource.gcloud_check,
    google_project_iam_member.cloudbuild-secrets,
    google_project_iam_member.cloudbuild-sql,
  ]
}

resource "google_secret_manager_secret_version" "db-pwd-initial" {
  provider    = google-beta
  secret      = google_secret_manager_secret.db-pwd.id
  secret_data = google_sql_user.user.password
}

resource "random_string" "bucket-name" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_storage_bucket" "export" {
  name               = "exposure-notification-export-${random_string.bucket-name.result}"
  bucket_policy_only = true
}

# This step automatically runs a build as well, so everything that uses an image depends on it.
resource "google_cloudbuild_trigger" "build-and-publish" {
  provider = google-beta
  count    = var.use_build_triggers ? 1 : 0

  name        = "build-containers"
  description = "Build the containers for the exposure notification service and deploy them to cloud run"
  filename    = "builders/deploy.yaml"
  github {
    owner = var.repo_owner
    name  = var.repo_name
    push {
      branch = "^master$"
    }
  }

  depends_on = [google_project_service.services["cloudbuild.googleapis.com"]]
}

# "build" does first time setup - it is different from "deploy" which we set up to trigger for later.
resource "null_resource" "submit-build-and-publish" {
  provisioner "local-exec" {
    command = "gcloud builds submit ../ --config ../builders/build.yaml --project ${data.google_project.project.project_id}"
  }

  depends_on = [
    null_resource.gcloud_check,
    google_project_iam_member.cloudbuild-secrets,
    google_project_iam_member.cloudbuild-sql,
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
      value = "10"
    },
    {
      name  = "DB_PASSWORD"
      value = "secret://${google_secret_manager_secret_version.db-pwd-initial.name}"
    },
    {
      # NOTE: We disable SSL here because the Cloud Run services use the Cloud
      # SQL proxy which runs on localhost. The proxy still uses a secure
      # connection to Cloud SQL.
      name  = "DB_SSLMODE"
      value = "disable"
    },
    {
      name  = "DB_HOST"
      value = "/cloudsql/${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
    },
    {
      name  = "DB_USER"
      value = google_sql_user.user.name
    },
    {
      name  = "DB_NAME"
      value = google_sql_database.db.name
    },
  ]
}

data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

# Cloud Scheduler requires AppEngine projects!
resource "google_app_engine_application" "app" {
  project     = data.google_project.project.project_id
  location_id = var.appengine_location
}
