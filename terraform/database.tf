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

resource "random_string" "db-name" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_sql_database_instance" "db-inst" {
  project          = data.google_project.project.project_id
  region           = local.db_region
  database_version = "POSTGRES_11"
  name             = "en-${random_string.db-name.result}"

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
    # This prevents accidential deletion of the database.
    prevent_destroy = true

    # Earlier versions of the database had a different name, and its not
    # possible to rename Cloud SQL instances.
    ignore_changes = [
      name,
    ]
  }

  depends_on = [
    google_project_service.services["sql-component.googleapis.com"],
  ]
}

resource "google_sql_ssl_cert" "client_cert" {
  common_name = "apollo"
  instance    = google_sql_database_instance.db-inst.name
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

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-pwd-initial" {
  provider    = google-beta
  secret      = google_secret_manager_secret.db-pwd.id
  secret_data = google_sql_user.user.password
}

# Grant Cloud Build the ability to access the database password (required to run
# migrations).
resource "google_secret_manager_secret_iam_member" "cloudbuild-db-pwd" {
  provider  = google-beta
  secret_id = google_secret_manager_secret.db-pwd.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

# Grant Cloud Build the ability to connect to Cloud SQL.
resource "google_project_iam_member" "cloudbuild-sql" {
  project = data.google_project.project.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [google_project_service.services["cloudbuild.googleapis.com"]]
}

# Migrate runs the initial database migrations.
resource "null_resource" "migrate" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID     = data.google_project.project.project_id
      DB_CONN        = google_sql_database_instance.db-inst.connection_name
      DB_PASS_SECRET = google_secret_manager_secret_version.db-pwd-initial.name
      DB_NAME        = google_sql_database.db.name
      DB_USER        = google_sql_user.user.name
      COMMAND        = "up"

      REGION   = local.db_region
      SERVICES = "all"
      TAG      = "initial"
    }

    command = "${path.module}/../scripts/migrate"
  }

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_secret_manager_secret_iam_member.cloudbuild-db-pwd,
    google_project_iam_member.cloudbuild-sql,
  ]
}

output "db_conn" {
  value = google_sql_database_instance.db-inst.connection_name
}

output "db_name" {
  value = google_sql_database.db.name
}

output "db_user" {
  value = google_sql_user.user.name
}

output "db_pass_secret" {
  value = google_secret_manager_secret_version.db-pwd-initial.name
}
