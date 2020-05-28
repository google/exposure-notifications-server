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

resource "google_service_account" "exposure" {
  project      = data.google_project.project.project_id
  account_id   = "en-exposure-sa"
  display_name = "Exposure Notification Exposure"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-exposure" {
  service_account_id = google_service_account.exposure.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "exposure-db" {
  provider = google-beta

  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.exposure.email}"
}

resource "google_cloud_run_service" "exposure" {
  name     = "exposure"
  location = var.region

  template {
    spec {
      service_account_name = google_service_account.exposure.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/exposure:initial"

        resources {
          limits = {
            cpu    = "2"
            memory = "1G"
          }
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
        "autoscaling.knative.dev/maxScale" : "50",
        "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.exposure-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template,
    ]
  }
}

resource "google_cloud_run_service_iam_member" "exposure-public" {
  location = google_cloud_run_service.exposure.location
  project  = google_cloud_run_service.exposure.project
  service  = google_cloud_run_service.exposure.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
