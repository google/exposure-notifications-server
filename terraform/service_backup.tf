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

resource "google_service_account" "backup" {
  project      = data.google_project.project.project_id
  account_id   = "en-backup-sa"
  display_name = "Exposure Notification Backup"
}

resource "google_service_account_iam_member" "cloudbuild-deploy-backup" {
  service_account_id = google_service_account.backup.id
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_iam_member" "backup-db" {
  for_each = toset([
    "sslcert",
    "sslkey",
    "sslrootcert",
    "password",
  ])

  secret_id = google_secret_manager_secret.db-secret[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.backup.email}"
}

resource "google_project_iam_member" "backup-observability" {
  for_each = toset([
    "roles/cloudtrace.agent",
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/stackdriver.resourceMetadata.writer",
  ])

  project = var.project
  role    = each.key
  member  = "serviceAccount:${google_service_account.backup.email}"
}

# Give the service account the ability to initiate backups - unfortunately this
# has to be granted at the project level and isn't scoped to a single database
# instance.
resource "google_project_iam_member" "database-backuper-cloudsql-viewer" {
  project = var.project
  role    = "roles/cloudsql.viewer"
  member  = "serviceAccount:${google_service_account.backup.email}"
}

resource "google_cloud_run_service" "backup" {
  name     = "backup"
  location = var.cloudrun_location

  autogenerate_revision_name = true

  metadata {
    annotations = merge(
      local.default_service_annotations,
      var.default_service_annotations_overrides,
      lookup(var.service_annotations, "backup", {}),
    )
  }

  template {
    spec {
      service_account_name = google_service_account.backup.email
      timeout_seconds      = 900

      containers {
        image = "gcr.io/${data.google_project.project.project_id}/github.com/google/exposure-notifications-server/backup:initial"

        resources {
          limits = {
            cpu    = "1"
            memory = "512Mi"
          }
        }

        dynamic "env" {
          for_each = merge(
            local.common_cloudrun_env_vars,

            {
              BACKUP_BUCKET                = google_storage_bucket.backups.name,
              BACKUP_DATABASE_INSTANCE_URL = google_sql_database_instance.db-inst.self_link,
              BACKUP_DATABASE_NAME         = google_sql_database.db.name,
            },

            // This MUST come last to allow overrides!
            lookup(var.service_environment, "_all", {}),
            lookup(var.service_environment, "backup", {}),
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
        lookup(var.revision_annotations, "backup", {}),
      )
    }
  }

  depends_on = [
    google_project_service.services["run.googleapis.com"],
    google_secret_manager_secret_iam_member.backup-db,
    null_resource.build,
    null_resource.migrate,
  ]

  lifecycle {
    ignore_changes = [
      metadata[0].annotations["client.knative.dev/user-image"],
      metadata[0].annotations["run.googleapis.com/client-name"],
      metadata[0].annotations["run.googleapis.com/client-version"],
      metadata[0].annotations["run.googleapis.com/ingress-status"],
      metadata[0].annotations["serving.knative.dev/creator"],
      metadata[0].annotations["serving.knative.dev/lastModifier"],
      metadata[0].labels["cloud.googleapis.com/location"],
      template[0].metadata[0].annotations["client.knative.dev/user-image"],
      template[0].metadata[0].annotations["run.googleapis.com/client-name"],
      template[0].metadata[0].annotations["run.googleapis.com/client-version"],
      template[0].metadata[0].annotations["serving.knative.dev/creator"],
      template[0].metadata[0].annotations["serving.knative.dev/lastModifier"],
      template[0].spec[0].containers[0].image,
    ]
  }
}


#
# Create scheduler job to invoke the service on a fixed interval.
#

resource "google_service_account" "backup-invoker" {
  project      = data.google_project.project.project_id
  account_id   = "en-backup-invoker-sa"
  display_name = "Exposure Notification Backup Invoker"
}

resource "google_cloud_run_service_iam_member" "backup-invoker" {
  project  = google_cloud_run_service.backup.project
  location = google_cloud_run_service.backup.location
  service  = google_cloud_run_service.backup.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.backup-invoker.email}"
}

resource "google_cloud_scheduler_job" "backup-worker" {
  name             = "backup-worker"
  region           = var.cloudscheduler_location
  schedule         = "0 */4 * * *"
  time_zone        = "America/Los_Angeles"
  attempt_deadline = "${google_cloud_run_service.backup.template[0].spec[0].timeout_seconds + 60}s"

  retry_config {
    retry_count = 3
  }

  http_target {
    http_method = "POST"
    uri         = "${google_cloud_run_service.backup.status.0.url}/"
    oidc_token {
      audience              = google_cloud_run_service.backup.status.0.url
      service_account_email = google_service_account.backup-invoker.email
    }
  }

  depends_on = [
    google_app_engine_application.app,
    google_cloud_run_service_iam_member.backup-invoker,
    google_project_service.services["cloudscheduler.googleapis.com"],
  ]
}
