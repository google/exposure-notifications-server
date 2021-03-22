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

resource "google_sql_database_instance" "db-inst" {
  project          = data.google_project.project.project_id
  region           = var.db_location
  database_version = var.db_version

  settings {
    tier              = var.cloudsql_tier
    disk_size         = var.cloudsql_disk_size_gb
    availability_type = "REGIONAL"

    database_flags {
      name  = "autovacuum"
      value = "on"
    }

    database_flags {
      name  = "max_connections"
      value = var.cloudsql_max_connections
    }

    database_flags {
      name  = "cloudsql.enable_pgaudit"
      value = "on"
    }

    database_flags {
      name  = "pgaudit.log"
      value = "all"
    }

    backup_configuration {
      enabled    = true
      location   = var.cloudsql_backup_location
      start_time = "02:00"
    }

    maintenance_window {
      day          = 7
      hour         = 2
      update_track = "stable"
    }

    ip_configuration {
      require_ssl     = true
      private_network = google_service_networking_connection.private_vpc_connection.network
    }

    insights_config {
      query_insights_enabled  = true
      query_string_length     = 1024
      record_application_tags = true
      record_client_address   = false
    }
  }

  depends_on = [
    google_project_service.services["sql-component.googleapis.com"],
  ]
}

resource "google_sql_database_instance" "replicas" {
  for_each = toset(var.db_failover_replica_regions)

  project          = var.project
  region           = each.key
  database_version = var.db_version

  master_instance_name = google_sql_database_instance.db-inst.name

  // These are REGIONAL replicas, which cannot auto-failover. The default
  // configuration has auto-failover in zones. This is for super disaster
  // recovery in which an entire region is down for an extended period of time.
  replica_configuration {
    failover_target = false
  }

  settings {
    tier              = var.cloudsql_tier
    disk_size         = var.cloudsql_disk_size_gb
    availability_type = "ZONAL"
    pricing_plan      = "PACKAGE"

    database_flags {
      name  = "autovacuum"
      value = "on"
    }

    database_flags {
      name  = "max_connections"
      value = var.cloudsql_max_connections
    }

    ip_configuration {
      require_ssl     = true
      private_network = google_service_networking_connection.private_vpc_connection.network
    }
  }

  depends_on = [
    google_project_service.services["sqladmin.googleapis.com"],
    google_project_service.services["sql-component.googleapis.com"],
  ]
}

resource "google_sql_database" "db" {
  project  = data.google_project.project.project_id
  instance = google_sql_database_instance.db-inst.name
  name     = var.db_name
}

resource "google_sql_ssl_cert" "db-cert" {
  project     = data.google_project.project.project_id
  instance    = google_sql_database_instance.db-inst.name
  common_name = "expsoure-notification"
}

resource "random_password" "db-password" {
  length  = 64
  special = false
}

resource "google_sql_user" "user" {
  instance = google_sql_database_instance.db-inst.name
  name     = var.db_user
  password = random_password.db-password.result
}

resource "google_secret_manager_secret" "db-secret" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = "db-${each.key}"

  replication {
    automatic = true
  }

  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "db-secret-version" {
  for_each = {
    sslcert     = google_sql_ssl_cert.db-cert.cert
    sslkey      = google_sql_ssl_cert.db-cert.private_key
    sslrootcert = google_sql_ssl_cert.db-cert.server_ca_cert
    password    = google_sql_user.user.password
  }

  secret      = google_secret_manager_secret.db-secret[each.key].id
  secret_data = each.value
}

# Grant Cloud Build the ability to access the database password (required to run
# migrations).
resource "google_secret_manager_secret_iam_member" "cloudbuild-db-pwd" {
  secret_id = google_secret_manager_secret.db-secret["password"].id
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

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"]
  ]
}

# Migrate runs the initial database migrations.
resource "null_resource" "migrate" {
  provisioner "local-exec" {
    environment = {
      PROJECT_ID  = data.google_project.project.project_id
      DB_CONN     = google_sql_database_instance.db-inst.connection_name
      DB_PASSWORD = "secret://${google_secret_manager_secret_version.db-secret-version["password"].name}"
      DB_NAME     = google_sql_database.db.name
      DB_USER     = google_sql_user.user.name

      TAG = "initial"
    }

    command = "${path.module}/../scripts/migrate"
  }

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
    google_secret_manager_secret_iam_member.cloudbuild-db-pwd,
    google_project_iam_member.cloudbuild-sql,
    null_resource.build,
  ]
}

# Create a storage bucket where database backups will be housed.
resource "google_storage_bucket" "backups" {
  project  = var.project
  name     = "${var.project}-backups"
  location = var.storage_location

  force_destroy               = true
  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      num_newer_versions = "120" // Default backup is 4x/day * 30 days
    }
  }

  depends_on = [
    google_project_service.services["storage.googleaips.com"],
  ]
}

# Give Cloud SQL the ability to create and manage backups.
resource "google_storage_bucket_iam_member" "instance-objectAdmin" {
  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_sql_database_instance.db-inst.service_account_email_address}"
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

output "db_inst_name" {
  value = google_sql_database_instance.db-inst.name
}

output "db_password" {
  value = google_secret_manager_secret_version.db-secret-version["password"].name
}

output "proxy_command" {
  value = "cloud_sql_proxy -dir \"$${HOME}/sql\" -instances=${google_sql_database_instance.db-inst.connection_name}=tcp:5432"
}

output "proxy_env" {
  value = "DB_SSLMODE=disable DB_HOST=127.0.0.1 DB_NAME=${google_sql_database.db.name} DB_PORT=5432 DB_USER=${google_sql_user.user.name} DB_PASSWORD=$(gcloud secrets versions access ${google_secret_manager_secret_version.db-secret-version["password"].name})"
}

output "psql_env" {
  value = "PGHOST=127.0.0.1 PGPORT=5432 PGUSER=${google_sql_user.user.name} PGPASSWORD=$(gcloud secrets versions access ${google_secret_manager_secret_version.db-secret-version["password"].name})"
}
