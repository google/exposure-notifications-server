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

#
# Create and deploy the service
#

resource "google_service_account" "export" {
  project      = data.google_project.project.project_id
  account_id   = "en-export-sa"
  display_name = "Exposure Notification Export"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-export" {
  service_account_id = google_service_account.export.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_project_iam_member" "export-cloudsql" {
  project = data.google_project.project.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.export.email}"
}

resource "google_secret_manager_secret_iam_member" "export-db-pwd" {
  provider = google-beta

  secret_id = google_secret_manager_secret.db-pwd.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.export.email}"
}

resource "google_storage_bucket_iam_member" "export-objectadmin" {
  bucket = google_storage_bucket.export.name
  role   = "roles/storage.objectAdmin" // overwrite is not included in objectCreator
  member = "serviceAccount:${google_service_account.export.email}"
}

resource "google_kms_key_ring_iam_member" "export-signerverifier" {
  key_ring_id = google_kms_key_ring.export-signing.self_link
  role        = "roles/cloudkms.signerVerifier"
  member      = "serviceAccount:${google_service_account.export.email}"
}

resource "google_cloud_run_service" "export" {
  name     = "export"
  location = var.region

  template {
    spec {
      service_account_name = google_service_account.export.email

      containers {
        image = "us.gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/export:latest"

        resources {
          limits = {
            cpu    = "2"
            memory = "1G"
          }
        }

        env {
          name  = "EXPORT_FILE_MAX_RECORDS"
          value = "100"
        }

        env {
          name  = "EXPORT_BUCKET"
          value = google_storage_bucket.export.name
        }

        dynamic "env" {
          for_each = local.common_cloudrun_env_vars
          content {
            name  = env.value["name"]
            value = env.value["value"]
          }
        }
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" : "1000",
        "run.googleapis.com/cloudsql-instances" : google_sql_database_instance.db-inst.connection_name
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_project_service.services["sqladmin.googleapis.com"],
    google_cloudbuild_trigger.build-and-publish,
  ]
}


#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "export-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-export-invoker-sa"
  display_name = "Exposure Notification Export Invoker"
}

resource "google_cloud_run_service_iam_member" "export-invoker" {
  project  = google_cloud_run_service.export.project
  location = google_cloud_run_service.export.location
  service  = google_cloud_run_service.export.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.export-invoker.email}"
}

resource "google_cloud_scheduler_job" "export-worker" {
  name             = "export-worker"
  schedule         = "* * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.export.status.0.url}/do-work"
    oidc_token {
      audience              = google_cloud_run_service.export.status.0.url
      service_account_email = google_service_account.export-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.export-invoker,
  ]
}

resource "google_cloud_scheduler_job" "export-create-batches" {
  name             = "export-create-batches"
  schedule         = "*/5 * * * *"
  time_zone        = "Etc/UTC"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = "${google_cloud_run_service.export.status.0.url}/create-batches"
    oidc_token {
      audience              = google_cloud_run_service.export.status.0.url
      service_account_email = google_service_account.export-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.export-invoker,
  ]
}
