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

resource "null_resource" "manual-step-to-enable-workspace" {
  # TODO: remove once
  # https://github.com/hashicorp/terraform-provider-google/issues/2605 is
  # fixed.
  provisioner "local-exec" {
    command = <<EOF
    echo -e '>>>> WARNING WARNING WARNING!'
    echo -e '>>>> Please use https://console.cloud.google.com/monitoring/signup?project=${var.project}&nextPath=monitoring to create the first workspace.'
    echo -e '>>>> Terraform cannot create workspace yet, you can only create workspace via the Google Cloud Console.'
    echo -e '>>>> Related doc: https://cloud.google.com/monitoring/workspaces/create#single-project-workspace'
    EOF
  }
}