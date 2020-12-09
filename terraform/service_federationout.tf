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

resource "google_service_account_iam_member" "cloudbuild-deploy-federationout" {
  service_account_id = google_service_account.federationout.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "federationout-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.federationout.email}"
}

resource "google_project_iam_member" "federationout-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.federationout.email}"
}

resource "google_cloud_run_service" "federationout" {
  name     = "federationout"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  template {
    spec {
      service_account_name = google_service_account.federationout.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/federationout:initial"

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
            lookup(var.service_environment, "federationout", {}),
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
        "autoscaling.knative.dev/maxScale" : "5",
        "run.googleapis.com/vpc-access-connector" : google_vpc_access_connector.connector.id
      }
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.federationout-db,
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

resource "google_cloud_run_service_iam_member" "federationout-public" {
  location = google_cloud_run_service.federationout.location
  project  = google_cloud_run_service.federationout.project
  service  = google_cloud_run_service.federationout.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}

#
# Custom domains and load balancer
#

resource "google_compute_region_network_endpoint_group" "federationout" {
  count = length(var.federationout_hosts) > 0 ? 1 : 0

  name     = "federationout"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.federationout.name
  }
}

resource "google_compute_backend_service" "federationout" {
  count = length(var.federationout_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "federationout"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.federationout[0].id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
}

output "federationout_urls" {
  value = concat([google_cloud_run_service.federationout.status.0.url], formatlist("https://%s", var.federationout_hosts))
}
