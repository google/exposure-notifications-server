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
  # The appengine_location depends on the deployment region. Here we define a
  # map to derive the appengine location from the region
  appengine_location = {
    asia-northeast1 = "asia-northeast1"
    europe-west1 = "europe-west"
    us-central1 = "us-central"
    us-east1 = "us-east1"
    us-east4 = "us-east4"
  }
}

