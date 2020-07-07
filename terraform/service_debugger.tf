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

resource "google_service_account" "debugger" {
  project      = data.google_project.project.project_id
  account_id   = "en-debugger-sa"
  display_name = "Exposure Notification debugger"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-debugger" {
  service_account_id = google_service_account.debugger.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "debugger-db" {
  provider = google-beta

  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.debugger.email}"
}

resource "google_project_iam_member" "debugger-run-viewer" {
  project = google_cloud_run_service.generate.project
  role    = "roles/run.viewer"
  member  = "serviceAccount:${google_service_account.debugger.email}"
}

resource "google_project_iam_member" "debugger-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.debugger.email}"
}

resource "google_cloud_run_service" "debugger" {
  count = var.deploy_debugger ? 1 : 0

  name     = "debugger"
  location = var.cloudrun_location

  template {
    spec {
      service_account_name = google_service_account.debugger.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/debugger:initial"

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

        dynamic "env" {
          for_each = lookup(var.service_environment, "debugger", {})
          content {
            name  = env.key
            value = env.value
          }
        }
      }

      container_concurrency = 10
    }

    metadata {
      annotations = {
        "autoscaling.knative.dev/maxScale" : "10",
        "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.debugger-db,
    null_resource.build,
  ]

  lifecycle {
    ignore_changes = [
      template,
    ]
  }
}
