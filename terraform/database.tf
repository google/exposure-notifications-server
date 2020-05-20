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
  region           = var.region
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
  }

  depends_on = [
    google_project_service.services["sql-component.googleapis.com"],
  ]
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

resource "google_secret_manager_secret_version" "db-pwd-initial" {
  provider    = google-beta
  secret      = google_secret_manager_secret.db-pwd.id
  secret_data = google_sql_user.user.password
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
    google_project_iam_member.cloudbuild-secrets,
    google_project_iam_member.cloudbuild-sql,
  ]
}
