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
    command = "gcloud projects describe ${var.project} || (echo 'please sign in to gcloud using `gcloud auth login`.' && exit 1)"
  }
}

data "google_project" "project" {
  project_id = var.project
  depends_on = [null_resource.gcloud_check]
}

resource "google_project_service" "services" {
  project = data.google_project.project.project_id
  for_each = toset(["run.googleapis.com", "cloudkms.googleapis.com", "secretmanager.googleapis.com", "storage-api.googleapis.com", "cloudscheduler.googleapis.com",
  "sql-component.googleapis.com", "cloudbuild.googleapis.com", "servicenetworking.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}

# TODO(ndmckinley): configure these service accounts to do the jobs they are designed for.
resource "google_service_account" "svc_acct" {
  project      = data.google_project.project.project_id
  account_id   = each.key
  display_name = each.value
  for_each = {
    "publisher" : "Publish Service Account",
    "exporter" : "Export Service Account",
    "fed-recv" : "Federation Receiver Service Account",
    "fed-pull" : "Federation Puller Service Account",
    "cleanup" : "Cleanup Service Account",
    "export-cleanup" : "Export Cleanup Service Account",
    "scheduler-cleanup" : "Cleanup Scheduler Service Account",
    "scheduler-export" : "Export Scheduler Service Account",
  }
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
    tier              = "db-custom-1-3840" # "db-custom-32-122880"
    availability_type = "REGIONAL"
    disk_size         = 500
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
  special = true
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
  # TODO(ndmckinley) is this the best way to get the schema into the database?
  # provisioner "local-exec" {
  #  command = "gcloud sql connect ${google_sql_database_instance.db-inst.name} -d ${google_sql_database.db.name} -u root --project ${data.google_project.project.project_id} < ../migrations/*.sql"
  # }
}

resource "google_secret_manager_secret" "db-pwd" {
  provider  = google-beta
  secret_id = "db-password"
  replication {
    automatic = true
  }
  depends_on = [google_project_service.services["secretmanager.googleapis.com"]]
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
  name = "exposure-notification-export-${random_string.bucket-name.result}"
}

# This step automatically runs a build as well, so everything that uses an image depends on it.
resource "google_cloudbuild_trigger" "build-and-publish" {
  provider    = google-beta
  name        = "build-containers"
  description = "Build the containers for the exposure notification service and deploy them to cloud run"
  filename    = "builders/deploy.yaml"
  github {
    owner = "google"
    name  = "exposure-notifications-server"
    push {
      branch = "^master$"
    }
  }
  provisioner "local-exec" {
    # "build" does first time setup - it is different from "deploy" which we set up to trigger for later.
    command = "gcloud builds submit ../ --config ../builders/build.yaml --project ${data.google_project.project.project_id}"
  }
}

resource "google_cloud_run_service" "exposure" {
  name     = "exposure"
  location = var.region
  template {
    spec {
      containers {
        image = "us.gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/exposure:latest"
        env {
          name  = "CONFIG_REFRESH_DURATION"
          value = "5m"
        }
        env {
          name  = "DB_POOL_MIN_CONNS"
          value = "2"
        }
        env {
          name  = "DB_POOL_MAX_CONNS"
          value = "10"
        }
        env {
          name  = "DB_PASSWORD_SECRET"
          value = google_secret_manager_secret_version.db-pwd-initial.name
        }
        env {
          name  = "DB_SSLKEY_SECRET"
          value = google_secret_manager_secret_version.db-key.name
        }
        env {
          name  = "DB_SSLCERT_SECRET"
          value = google_secret_manager_secret_version.db-cert.name
        }
        env {
          name  = "DB_SSLROOTCERT_SECRET"
          value = google_secret_manager_secret_version.db-ca-cert.name
        }
        env {
          name  = "DB_SSLMODE"
          value = "require"
        }
        env {
          name  = "DB_HOST"
          value = "/cloudsql/${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
        }
        env {
          name  = "DB_USER"
          value = google_sql_user.user.name
        }
        env {
          name  = "DB_DBNAME"
          value = google_sql_database.db.name
        }
      }
    }
    metadata {
      annotations = {
        "run.googleapis.com/cloudsql-instances" : "${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
        "autoscaling.knative.dev/maxScale"      : "1000"
      }
    }
  }
	depends_on = [google_cloudbuild_trigger.build-and-publish]
}

resource "google_cloud_run_service" "export" {
  name     = "export"
  location = var.region
  template {
    spec {
      containers {
        image = "us.gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/export:latest"
        env {
          name  = "EXPORT_FILE_MAX_RECORDS"
          value = "100"
        }
        env {
          name  = "EXPORT_BUCKET"
          value = google_storage_bucket.export.name
        }
        env {
          name  = "DB_POOL_MIN_CONNS"
          value = "2"
        }
        env {
          name  = "DB_POOL_MAX_CONNS"
          value = "10"
        }
        env {
          name  = "DB_PASSWORD_SECRET"
          value = google_secret_manager_secret_version.db-pwd-initial.name
        }
        env {
          name  = "DB_SSLKEY_SECRET"
          value = google_secret_manager_secret_version.db-key.name
        }
        env {
          name  = "DB_SSLCERT_SECRET"
          value = google_secret_manager_secret_version.db-cert.name
        }
        env {
          name  = "DB_SSLROOTCERT_SECRET"
          value = google_secret_manager_secret_version.db-ca-cert.name
        }
        env {
          name  = "DB_SSLMODE"
          value = "require"
        }
        env {
          name  = "DB_HOST"
          value = "/cloudsql/${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
        }
        env {
          name  = "DB_USER"
          value = google_sql_user.user.name
        }
        env {
          name  = "DB_DBNAME"
          value = google_sql_database.db.name
        }
      }
    }
    metadata {
      annotations = {
        "run.googleapis.com/cloudsql-instances" : "${data.google_project.project.project_id}:${var.region}:${google_sql_database_instance.db-inst.name}"
        "autoscaling.knative.dev/maxScale"      : "1000"
      }
    }
  }
	depends_on = [google_cloudbuild_trigger.build-and-publish]
}



data "google_iam_policy" "noauth" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_service_iam_policy" "export-noauth" {
  location = google_cloud_run_service.exposure.location
  project  = google_cloud_run_service.exposure.project
  service  = google_cloud_run_service.exposure.name

  policy_data = data.google_iam_policy.noauth.policy_data
}

resource "google_cloud_run_service_iam_policy" "exposure-noauth" {
  location = google_cloud_run_service.exposure.location
  project  = google_cloud_run_service.exposure.project
  service  = google_cloud_run_service.exposure.name

  policy_data = data.google_iam_policy.noauth.policy_data
}

resource "google_project_iam_member" "export-worker" {
  project = data.google_project.project.project_id
  role    = "roles/run.invoker"
  member  = "serviceAccount:${google_service_account.svc_acct["scheduler-export"].email}"
}

resource "google_cloud_scheduler_job" "export-worker" {
  name             = "export-worker"
  schedule         = "* * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "60s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.export.status.0.url}/do-work"
    oidc_token {
      service_account_email = google_service_account.svc_acct["scheduler-export"].email
    }
  }
  depends_on = [google_project_iam_member.export-worker]
}

resource "google_cloud_scheduler_job" "export-create-batches" {
  name             = "export-create-batches"
  schedule         = "* * * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "60s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.export.status.0.url}/create-batches"
    oidc_token {
      service_account_email = google_service_account.svc_acct["scheduler-export"].email
    }
  }
  depends_on = [google_project_iam_member.export-worker]
}
