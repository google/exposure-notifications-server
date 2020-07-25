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
# Create necessary generated secretes
#

resource "random_id" "revision_token_aad" {
  byte_length = 32
}

resource "google_secret_manager_secret" "revision_token_aad" {
  provider = google-beta

  secret_id = "revision-token-aad"
  replication {
    automatic = true
  }
  depends_on = [
    google_project_service.services["secretmanager.googleapis.com"],
  ]
}

resource "google_secret_manager_secret_version" "revision_token_aad_secret_version" {
  provider = google-beta

  secret      = google_secret_manager_secret.revision_token_aad.id
  secret_data = random_id.revision_token_aad.b64_std
}
