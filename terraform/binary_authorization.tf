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

resource "google_binary_authorization_policy" "policy" {
  project = var.project

  global_policy_evaluation_mode = "ENABLE"

  default_admission_rule {
    evaluation_mode  = "REQUIRE_ATTESTATION"
    enforcement_mode = var.binary_authorization_enforcement_mode
    require_attestations_by = [
      google_binary_authorization_attestor.built-by-ci.name,
    ]
  }

  dynamic "admission_whitelist_patterns" {
    for_each = var.binary_authorization_allowlist_patterns
    content {
      name_pattern = admission_whitelist_patterns.value
    }
  }

  depends_on = [
    google_project_service.services["binaryauthorization.googleapis.com"],
  ]
}

resource "google_container_analysis_note" "built-by-ci" {
  project = var.project
  name    = "built-by-ci"

  attestation_authority {
    hint {
      human_readable_name = "Built by continuous integration"
    }
  }

  depends_on = [
    google_project_service.services["containeranalysis.googleapis.com"],
  ]
}

resource "google_binary_authorization_attestor" "built-by-ci" {
  project = var.project
  name    = "built-by-ci"

  attestation_authority_note {
    note_reference = google_container_analysis_note.built-by-ci.name

    public_keys {
      id = data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.id
      pkix_public_key {
        public_key_pem      = data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.public_key[0].pem
        signature_algorithm = data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.public_key[0].algorithm
      }
    }
  }

  depends_on = [
    google_project_service.services["binaryauthorization.googleapis.com"],
  ]
}

# Unfortunately Terraform does not have the ability to attach IAM permissions to
# the note. Granting this at the project level isn't ideal, but it is acceptable
# given the few notes in this project.
resource "google_project_iam_member" "ci-notes" {
  project = var.project
  role    = "roles/containeranalysis.notes.attacher"
  member  = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_binary_authorization_attestor_iam_member" "ci-attestor" {
  project  = var.project
  attestor = google_binary_authorization_attestor.built-by-ci.id
  role     = "roles/binaryauthorization.attestorsViewer"
  member   = "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com"

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

resource "google_kms_key_ring" "binauthz-keyring" {
  project  = var.project
  name     = var.kms_binary_authorization_key_ring_name
  location = var.kms_location

  depends_on = [
    google_project_service.services["cloudkms.googleapis.com"],
  ]
}

resource "google_kms_crypto_key" "binauthz-built-by-ci-signer" {
  key_ring = google_kms_key_ring.binauthz-keyring.self_link
  name     = "binauthz-built-by-ci-signer"
  purpose  = "ASYMMETRIC_SIGN"

  version_template {
    algorithm        = "RSA_SIGN_PKCS1_4096_SHA512"
    protection_level = "HSM"
  }
}

data "google_kms_crypto_key_version" "binauthz-built-by-ci-signer-version" {
  crypto_key = google_kms_crypto_key.binauthz-built-by-ci-signer.self_link
}

resource "google_kms_crypto_key_iam_binding" "ci-attest" {
  crypto_key_id = google_kms_crypto_key.binauthz-built-by-ci-signer.id
  role          = "roles/cloudkms.signerVerifier"

  members = [
    "serviceAccount:${data.google_project.project.number}@cloudbuild.gserviceaccount.com",
  ]

  depends_on = [
    google_project_service.services["cloudbuild.googleapis.com"],
  ]
}

output "binary_authorization_key_id" {
  value = trimprefix(data.google_kms_crypto_key_version.binauthz-built-by-ci-signer-version.id, "//cloudkms.googleapis.com/v1/")
}

output "binary_authorization_attestor_id" {
  value = google_binary_authorization_attestor.built-by-ci.id
}
