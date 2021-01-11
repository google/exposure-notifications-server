# Copyright 2021 Google LLC
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

resource "google_service_account" "admin-console" {
  project      = data.google_project.project.project_id
  account_id   = "en-admin-console-sa"
  display_name = "Exposure Notification Admin Console"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-admin-console" {
  service_account_id = google_service_account.admin-console.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "admin-console-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.admin-console.email}"
}

resource "google_project_iam_member" "admin-console-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.admin-console.email}"
}

resource "google_cloud_run_service" "admin-console" {
  name     = "admin-console"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "admin-console", {}),
    )
  }
  template {
    spec {
      service_account_name = google_service_account.admin-console.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/admin-console:initial"

        resources {
          limits = {
            cpu    = "1000m"
            memory = "1G"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.common_cloudrun_env_vars,

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "admin-console", {}),
          )

          content {
            name  = env.key
            value = env.value
          }
        }
      }

      container_concurrency = 10
    }

    metadata {
      annotations = merge(
        local.default_revision_annotations,
        var.default_revision_annotations_overrides,
        lookup(var.revision_annotations, "admin-console", {}),
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.admin-console-db,
    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].spec[0].containers[0].image,
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].labels["cloud.googleapis.com/location"],
    ]
  }
}

resource "google_cloud_run_service_iam_member" "admin-console-invoker" {
  for_each = toset(var.admin_console_invokers)

  project  = google_cloud_run_service.admin-console.project
  location = google_cloud_run_service.admin-console.location
  service  = google_cloud_run_service.admin-console.name
  role     = "roles/run.invoker"
  member   = each.key
}

#
# Custom domains and load balancer
#

resource "google_compute_region_network_endpoint_group" "admin-console" {
  count = length(var.admin_console_hosts) > 0 ? 1 : 0

  name     = "admin-console"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.admin-console.name
  }
}

resource "google_compute_backend_service" "admin-console" {
  count = length(var.admin_console_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "admin-console"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.admin-console[0].id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
  log_config {
    enable = var.enable_lb_logging
  }
}

output "admin_console_urls" {
  value = concat([google_cloud_run_service.admin-console.status.0.url], formatlist("https://%s", var.admin_console_hosts))
}
