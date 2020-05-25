# Copyright 2020 Google LLC
# Copyright 2020 CriticalBlue Ltd.
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

locals {
  # Define locals that apply the project default for region if no other value is specified
  db_region    = replace(var.db_region, "/^default$/", var.region)
  kms_location = replace(var.kms_location, "/^default$/", var.region)
  appengine_location = replace(replace(replace(var.appengine_location, "/^default$/", var.region), "/^us-central1$/", "us-central"), "/^europe-west1$/", "europe-west")
  appengine_region = replace(replace(local.appengine_location, "/^us-central$/", "us-central1"), "/^europe-west$/", "europe-west1")
  cloudrun_location = replace(var.cloudrun_location, "/^default$/", var.region)
  storage_location = upper(replace(var.storage_location, "/^default$/", var.region))
}
