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

resource "google_monitoring_notification_channel" "paging" {
  provider     = google-beta
  project      = var.project
  display_name = "Paging Notification Channel"
  type         = each.key
  labels       = each.value.labels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]

  for_each = var.alert-notification-channel-paging
}

resource "google_monitoring_notification_channel" "non-paging" {
  provider     = google-beta
  project      = var.project
  display_name = "Non-paging Notification Channel"
  type         = each.key
  labels       = each.value.labels
  depends_on = [
    null_resource.manual-step-to-enable-workspace
  ]

  for_each = var.alert-notification-channel-non-paging
}
