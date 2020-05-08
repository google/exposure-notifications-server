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
  depends_on = [null_resource.n]
}

resource "google_project_service" "services" {
  project = data.google_project.project.project_id
  for_each = toset(["run.googleapis.com", "cloudkms.googleapis.com", "secretmanager.googleapis.com", "storage-api.googleapis.com", "cloudscheduler.googleapis.com",
  "sql-component.googleapis.com", "cloudbuild.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}

resource "google_service_account" "svc_acct" {
  project      = data.google_project.project.project_id
  account_id   = each.key
  display_name = each.value
  for_each = {
    "publisher" : "Publish Service Account",
    "exporter" : "Export Service Account",
    "fed-recv" : "Federation Receiver Service Account",
    "fed-pull" : "Federation Puller Service Account",
    "wipeout" : "Wipeout Service Account",
    "export-wipeout" : "Export Wipeout Service Account",
    "scheduler-wipeout" : "Wipeout Scheduler Service Account",
    "scheduler-export" : "Export Scheduler Service Account",
  }
}

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
  }
  lifecycle {
    prevent_destroy = true
  }
  depends_on = [google_project_service.services["sql-component.googleapis.com"]]
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
  provisioner "local-exec" {
    # TODO(ndmckinley) this isn't the most up-to-date version of the schema because of the migrations files.
    # TODO(ndmckinley) is this the best way to get the schema into the database?
    command = "gcloud sql connect ${google_sql_database_instance.db-inst.name} -d ${google_sql_database.db.name} -u root --project ${data.google_project.project.project_id} < ../scripts/schema.sql"
  }
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


# TODO(ndmckinley) build the binaries and deploy them.
