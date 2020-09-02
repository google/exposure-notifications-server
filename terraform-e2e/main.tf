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

variable "project" {
  type = string
}

resource "random_string" "suffix" {
  length  = 5
  special = false
  number  = false
  upper   = false
}

module "en" {
  source = "../terraform"

  project                           = var.project
  cloudsql_disk_size_gb             = 500
  db_name                           = "en-server-${random_string.suffix.result}"
  kms_export_signing_key_ring_name  = "export-signing-${random_string.suffix.result}"
  kms_revision_tokens_key_ring_name = "revision-tokens-${random_string.suffix.result}"

  create_env_file = true

  service_environment = {
    export = {
      LOG_DEBUG = "true"
    }

    exposure = {
      LOG_DEBUG = "true"
    }

    generate = {
      LOG_DEBUG = "true"
    }
  }
}

output "en" {
  value = module.en
}
