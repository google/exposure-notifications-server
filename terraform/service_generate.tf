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

resource "google_service_account" "generate" {
  project      = data.google_project.project.project_id
  account_id   = "en-generate-sa"
  display_name = "Exposure Notification Generate"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-generate" {
  service_account_id = google_service_account.generate.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "generate-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.generate.email}"
}

resource "google_project_iam_member" "generate-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.generate.email}"
}

resource "google_cloud_run_service" "generate" {
  name     = "generate"
  location = var.cloudrun_location

  template {
    spec {
      service_account_name = google_service_account.generate.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/cmd/generate:initial"

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
          for_each = lookup(var.service_environment, "generate", {})
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
    google_secret_manager_secret_iam_member.generate-db,
    null_resource.build,
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

resource "google_service_account" "generate-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-generate-invoker-sa"
  display_name = "Exposure Notification Generate Invoker"
}

resource "google_cloud_run_service_iam_member" "generate-invoker" {
  project  = google_cloud_run_service.generate.project
  location = google_cloud_run_service.generate.location
  service  = google_cloud_run_service.generate.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.generate-invoker.email}"
}

locals {
  generate_uri = length(var.generate_regions) > 0 ? "${google_cloud_run_service.generate.status.0.url}/?region=${join(",", var.generate_regions)}" : "${google_cloud_run_service.generate.status.0.url}/"
}

resource "google_cloud_scheduler_job" "generate-worker" {
  name             = "generate-worker"
  schedule         = var.generate_cron_schedule
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "60s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "GET"
    uri         = local.generate_uri
    oidc_token {
      audience              = google_cloud_run_service.generate.status.0.url
      service_account_email = google_service_account.generate-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.generate-invoker,
  ]
}
