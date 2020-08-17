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

resource "random_string" "key-suffix" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

resource "google_kms_key_ring" "export-signing" {
  project  = data.google_project.project.project_id
  name     = "export-signing-${random_string.key-suffix.result}"
  location = var.kms_location

  depends_on = [
    google_project_service.services["cloudkms.googleapis.com"],
  ]
}

resource "google_kms_crypto_key" "export-signer" {
  key_ring = google_kms_key_ring.export-signing.self_link
  name     = "signer"
  purpose  = "ASYMMETRIC_SIGN"
  version_template {
    algorithm        = "EC_SIGN_P256_SHA256"
    protection_level = "HSM"
  }
}

resource "google_kms_key_ring" "revision-tokens" {
  project  = data.google_project.project.project_id
  name     = "revision-tokens-${random_string.key-suffix.result}"
  location = var.kms_location

  depends_on = [
    google_project_service.services["cloudkms.googleapis.com"],
  ]
}

resource "google_kms_crypto_key" "token-key" {
  key_ring = google_kms_key_ring.revision-tokens.self_link
  name     = "token-key"
  purpose  = "ENCRYPT_DECRYPT"
  version_template {
    algorithm        = "GOOGLE_SYMMETRIC_ENCRYPTION"
    protection_level = "HSM"
  }
}

data "google_kms_crypto_key_version" "token_key_version" {
  crypto_key = google_kms_crypto_key.token-key.self_link
}