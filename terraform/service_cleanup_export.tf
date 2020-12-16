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

resource "google_service_account" "cleanup-export" {
  project      = data.google_project.project.project_id
  account_id   = "en-cleanup-export-sa"
  display_name = "Exposure Notification Cleanup Export"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-cleanup-export" {
  service_account_id = google_service_account.cleanup-export.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cleanup-export-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup-export.email}"
}

resource "google_storage_bucket_iam_member" "cleanup-export-objectadmin" {
  bucket = google_storage_bucket.export.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.cleanup-export.email}"
}

resource "google_project_iam_member" "cleanup-export-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.cleanup-export.email}"
}

resource "google_cloud_run_service" "cleanup-export" {
  name     = "cleanup-export"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.cleanup-export.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cleanup-export:initial"

        resources {
          limits = {
            cpu    = "2000m"
            memory = "2G"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.common_cloudrun_env_vars,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "cleanup_export", {}),
          )

          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" : "1",
        "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
        "run.googleapis.com/vpc-access-egress" : "private-ranges-only"
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.cleanup-export-db,
    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].spec[0].containers[0].image,
    ]
  }
}


#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "cleanup-export-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-cleanup-export-invoker-sa"
  display_name = "Exposure Notification Cleanup Export Invoker"
}

resource "google_cloud_run_service_iam_member" "cleanup-export-invoker" {
  project  = google_cloud_run_service.cleanup-export.project
  location = google_cloud_run_service.cleanup-export.location
  service  = google_cloud_run_service.cleanup-export.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cleanup-export-invoker.email}"
}

resource "google_cloud_scheduler_job" "cleanup-export-worker" {
  name             = "cleanup-export-worker"
  region           = var.cloudscheduler_location
  schedule         = var.cleanup_export_worker_cron_schedule
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.cleanup-export.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.cleanup-export.status.0.url
      service_account_email = google_service_account.cleanup-export-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.cleanup-export-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
