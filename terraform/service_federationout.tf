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

resource "google_service_account" "federationout" {
  project      = data.google_project.project.project_id
  account_id   = "en-federationout-sa"
  display_name = "Exposure Notification Federation (Out)"
}

resource "google_project_iam_member" "federationout-cloudsql" {
  project = data.google_project.project.project_id
  role    = "roles/cloudsql.client"
  member  = "serviceAccount:${google_service_account.federationout.email}"
}

resource "google_secret_manager_secret_iam_member" "federationout-db-pwd" {
  provider = google-beta

  secret_id = google_secret_manager_secret.db-pwd.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.federationout.email}"
}

resource "google_cloud_run_service" "federationout" {
  name     = "federationout"
  location = var.region

  template {
    spec {
      service_account_name = google_service_account.federationout.email

      containers {
        image = "us.gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/federationout:latest"

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
        "autoscaling.knative.dev/maxScale" : "1000",
        "run.googleapis.com/cloudsql-instances" : google_sql_database_instance.db-inst.connection_name
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_project_service.services["sqladmin.googleapis.com"],
    null_resource.submit-build-and-publish,
  ]
}

resource "google_cloud_run_service_iam_member" "federationout-public" {
  location = google_cloud_run_service.federationout.location
  project  = google_cloud_run_service.federationout.project
  service  = google_cloud_run_service.federationout.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
