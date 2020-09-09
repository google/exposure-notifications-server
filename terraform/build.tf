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

resource "google_storage_bucket" "cloudbuild-cache" {
  project  = var.project
  name     = "${var.project}-cloudbuild-cache"
  location = var.storage_location

  force_destroy               = true
  uniform_bucket_level_access = true

  // Automatically expire cached objects after 14 days.
  lifecycle_rule {
    action {
      type = "Delete"
    }

    condition {
      age = "14"
    }
  }

  depends_on = [
    google_project_service.services["storage.googleaips.com"],
  ]
}

resource "google_storage_bucket_iam_member" "cloudbuild-cache" {
  bucket = google_storage_bucket.cloudbuild-cache.name
  role   = "roles/storage.objectAdmin"
  member = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"
}
