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

resource "google_service_account" "cleanup-exposure" {
  project      = data.google_project.project.project_id
  account_id   = "en-cleanup-exposure-sa"
  display_name = "Exposure Notification Cleanup Exposure"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-cleanup-exposure" {
  service_account_id = google_service_account.cleanup-exposure.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "cleanup-exposure-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.cleanup-exposure.email}"
}

resource "google_project_iam_member" "cleanup-exposure-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.cleanup-exposure.email}"
}

resource "google_cloud_run_service" "cleanup-exposure" {
  name     = "cleanup-exposure"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.cleanup-exposure.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cleanup-exposure:initial"

        resources {
          limits = {
            cpu    = "2000m"
            memory = "1G"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.common_cloudrun_env_vars,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "cleanup_exposure", {}),
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
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.cleanup-exposure-db,
    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations,
      template[0].spec[0].containers[0].image,
    ]
  }
}


#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "cleanup-exposure-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-cleanup-exposure-invoker-sa"
  display_name = "Exposure Notification Cleanup Exposure Invoker"
}

resource "google_cloud_run_service_iam_member" "cleanup-exposure-invoker" {
  project  = google_cloud_run_service.cleanup-exposure.project
  location = google_cloud_run_service.cleanup-exposure.location
  service  = google_cloud_run_service.cleanup-exposure.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.cleanup-exposure-invoker.email}"
}

resource "google_cloud_scheduler_job" "cleanup-exposure-worker" {
  name             = "cleanup-exposure-worker"
  region           = var.cloudscheduler_location
  schedule         = var.cleanup_exposure_worker_cron_schedule
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "600s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.cleanup-exposure.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.cleanup-exposure.status.0.url
      service_account_email = google_service_account.cleanup-exposure-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.cleanup-exposure-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
