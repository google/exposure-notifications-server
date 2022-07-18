# Copyright 2020 the Exposure Notifications Server authors
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

resource "google_secret_manager_secret_iam_member" "revision-token-aad" {
  secret_id = google_secret_manager_secret.revision_token_aad.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.exposure.email}"
}

resource "google_kms_key_ring_iam_member" "revision-tokens-encrypt-decrypt" {
  key_ring_id = google_kms_key_ring.revision-tokens.id
  role        = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member      = "serviceAccount:${google_service_account.exposure.email}"
}

resource "google_project_iam_member" "exposure-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.exposure.email}"
}

resource "google_cloud_run_service" "exposure" {
  name     = "exposure"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "exposure", {}),
    )
  }

  template {
    spec {
      service_account_name = google_service_account.exposure.email

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/exposure:initial"

        resources {
          limits = {
            cpu    = "2000m"
            memory = "1G"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.common_cloudrun_env_vars,
            {
              "REVISION_TOKEN_KEY_ID" = google_kms_crypto_key.token-key.id
              "REVISION_TOKEN_AAD"    = "secret://${google_secret_manager_secret_version.revision_token_aad_secret_version.id}"
            },

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
            lookup(var.service_environment, "exposure", {}),
          )

          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }

    metadata {
      annotations = merge(
        local.default_revision_annotations,
        var.default_revision_annotations_overrides,
        lookup(var.revision_annotations, "exposure", {}),
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.exposure-db,
    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["client.knative.dev/user-image"],
      metadata[0].annotations["run.googleapis.com/client-name"],
      metadata[0].annotations["run.googleapis.com/client-version"],
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].annotations["run.googleapis.com/launch-stage"],
      metadata[0].annotations["serving.knative.dev/creator"],
      metadata[0].annotations["serving.knative.dev/lastModifier"],
      metadata[0].labels["cloud.googleapis.com/location"],
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].metadata[0].annotations["run.googleapis.com/sandbox"],
      template[0].metadata[0].annotations["serving.knative.dev/creator"],
      template[0].metadata[0].annotations["serving.knative.dev/lastModifier"],
      template[0].spec[0].containers[0].image,
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

#
# Custom domains and load balancer
#

resource "google_compute_region_network_endpoint_group" "exposure" {
  count = length(var.exposure_hosts) > 0 ? 1 : 0

  name     = "exposure"
  provider = google-beta
  project  = var.project
  region   = var.region

  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = google_cloud_run_service.exposure.name
  }
}

resource "google_compute_backend_service" "exposure" {
  count = length(var.exposure_hosts) > 0 ? 1 : 0

  provider = google-beta
  name     = "exposure"
  project  = var.project

  backend {
    group = google_compute_region_network_endpoint_group.exposure[0].id
  }
  security_policy = google_compute_security_policy.cloud-armor.name
  log_config {
    enable = var.enable_lb_logging
  }
}

output "exposure_urls" {
  value = concat([google_cloud_run_service.exposure.status.0.url], formatlist("https://%s", var.exposure_hosts))
}
