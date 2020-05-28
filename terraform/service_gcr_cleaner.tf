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

resource "google_service_account" "gcr-cleaner" {
  project      = data.google_project.project.project_id
  account_id   = "gcr-cleaner"
  display_name = "Container Registry Cleaner"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-gcr-cleaner" {
  service_account_id = google_service_account.gcr-cleaner.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_storage_bucket_iam_member" "gcr-cleaner-objectadmin" {
  bucket = "artifacts.${data.google_project.project.project_id}.appspot.com"
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${google_service_account.gcr-cleaner.email}"
}

resource "google_cloud_run_service" "gcr-cleaner" {
  name     = "gcr-cleaner"
  location = var.cloudrun_location

  template {
    spec {
      service_account_name = google_service_account.gcr-cleaner.email

      containers {
        image = "us-docker.pkg.dev/gcr-cleaner/gcr-cleaner/gcr-cleaner"
      }
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" : "3",
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
  ]

  lifecycle {
    ignore_changes = [
      template,
    ]
  }
}


#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "gcr-cleaner-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "gcr-cleaner-invoker"
  display_name = "Container Registry Cleaner Invoker"
}

resource "google_cloud_run_service_iam_member" "gcr-cleaner-invoker" {
  project  = google_cloud_run_service.gcr-cleaner.project
  location = google_cloud_run_service.gcr-cleaner.location
  service  = google_cloud_run_service.gcr-cleaner.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.gcr-cleaner-invoker.email}"
}

resource "google_cloud_scheduler_job" "gcr-cleaner-worker" {
  for_each = toset([
    "cleanup-export",
    "cleanup-exposure",
    "export",
    "exposure",
    "federationin",
    "federationout",
    "generate",
  ])

  name             = "gcr-cleaner-worker-${each.value}"
  region           = var.cloudscheduler_region
  schedule         = var.registry_cleanup_cron_schedule
  time_zone        = "Etc/UTC"
  attempt_deadline = "600s"

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.gcr-cleaner.status.0.url}/http"

    body = base64encode(jsonencode({
      repo           = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/${each.value}"
      grace          = "2h"
      allowed_tagged = true
      keep           = 3
    }))

    oidc_token {
      audience              = google_cloud_run_service.gcr-cleaner.status.0.url
      service_account_email = google_service_account.gcr-cleaner-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.gcr-cleaner-invoker,
  ]
}
