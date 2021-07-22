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

resource "random_string" "bucket-name" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_storage_bucket" "export" {
  project  = data.google_project.project.project_id
  location = var.storage_location
  name     = "exposure-notification-export-${random_string.bucket-name.result}"

  uniform_bucket_level_access = true

  versioning {
    enabled = true
  }

  lifecycle_rule {
    condition {
      num_newer_versions = "3"
    }

    action {
      type = "Delete"
    }
  }
}

resource "google_storage_bucket_iam_member" "public" {
  bucket = google_storage_bucket.export.name
  role   = "roles/storage.objectViewer"
  member = "allUsers"
}

resource "google_compute_backend_bucket" "export" {
  count = length(var.export_hosts) > 0 ? 1 : 0

  name        = "export-backend-bucket"
  bucket_name = google_storage_bucket.export.name
  enable_cdn  = var.enable_cdn_for_exports

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

output "export_bucket" {
  value = google_storage_bucket.export.name
}
