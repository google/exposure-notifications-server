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

locals {
  all_hosts = toset(concat(var.debugger_hosts, var.export_hosts, var.exposure_hosts, var.federationout_hosts))
  enable_lb = length(local.all_hosts) > 0
}

resource "google_compute_ssl_policy" "one-two-ssl-policy" {
  name            = "one-two-ssl-policy"
  profile         = "MODERN"
  min_tls_version = "TLS_1_2"

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_global_address" "key-server" {
  count = local.enable_lb ? 1 : 0

  name    = "key-server-address"
  project = var.project

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

# Redirects all requests to https
resource "google_compute_url_map" "urlmap-http" {
  count = local.enable_lb ? 1 : 0

  name     = "https-redirect"
  provider = google-beta
  project  = var.project

  default_url_redirect {
    strip_query    = false
    https_redirect = true
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_url_map" "urlmap-https" {
  count = local.enable_lb ? 1 : 0

  name            = "key-server"
  provider        = google-beta
  project         = var.project
  default_service = google_compute_backend_service.exposure[0].id

  // debugger
  dynamic "host_rule" {
    for_each = length(var.debugger_hosts) > 0 ? [1] : []

    content {
      path_matcher = "debugger"
      hosts        = var.debugger_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.debugger_hosts) > 0 ? [1] : []

    content {
      name            = "debugger"
      default_service = google_compute_backend_service.debugger[0].id
    }
  }

  // export
  dynamic "host_rule" {
    for_each = length(var.export_hosts) > 0 ? [1] : []

    content {
      path_matcher = "export"
      hosts        = var.export_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.export_hosts) > 0 ? [1] : []

    content {
      name            = "export"
      default_service = google_compute_backend_bucket.export[0].id
    }
  }

  // exposure
  dynamic "host_rule" {
    for_each = length(var.exposure_hosts) > 0 ? [1] : []

    content {
      path_matcher = "exposure"
      hosts        = var.exposure_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.exposure_hosts) > 0 ? [1] : []

    content {
      name            = "exposure"
      default_service = google_compute_backend_service.exposure[0].id
    }
  }

  // federationout
  dynamic "host_rule" {
    for_each = length(var.federationout_hosts) > 0 ? [1] : []

    content {
      path_matcher = "federationout"
      hosts        = var.federationout_hosts
    }
  }

  dynamic "path_matcher" {
    for_each = length(var.federationout_hosts) > 0 ? [1] : []

    content {
      name            = "federationout"
      default_service = google_compute_backend_service.federationout[0].id
    }
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

resource "google_compute_target_http_proxy" "http" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "key-server"
  project  = var.project

  url_map = google_compute_url_map.urlmap-http[0].id
}

resource "google_compute_target_https_proxy" "https" {
  count = local.enable_lb ? 1 : 0

  name    = "key-server"
  project = var.project

  url_map          = google_compute_url_map.urlmap-https[0].id
  ssl_certificates = [google_compute_managed_ssl_certificate.default[0].id]
  ssl_policy       = google_compute_ssl_policy.one-two-ssl-policy.id
}

resource "google_compute_global_forwarding_rule" "http" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "key-server-http"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.key-server[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "80"
  target                = google_compute_target_http_proxy.http[0].id
}

resource "google_compute_global_forwarding_rule" "https" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "key-server-https"
  project  = var.project

  ip_protocol           = "TCP"
  ip_address            = google_compute_global_address.key-server[0].address
  load_balancing_scheme = "EXTERNAL"
  port_range            = "443"
  target                = google_compute_target_https_proxy.https[0].id
}

resource "random_id" "certs" {
  count = local.enable_lb ? 1 : 0

  byte_length = 4

  keepers = {
    domains = join(",", local.all_hosts)
  }
}

resource "google_compute_managed_ssl_certificate" "default" {
  count = local.enable_lb ? 1 : 0

  provider = google-beta
  name     = "key-certificates-${random_id.certs[0].hex}"
  project  = var.project

  managed {
    domains = local.all_hosts
  }

  # This is to prevent destroying the cert while it's still attached to the load
  # balancer.
  lifecycle {
    create_before_destroy = true
  }

  depends_on = [
    google_project_service.services["compute.googleapis.com"],
  ]
}

output "lb_ip" {
  value = google_compute_global_address.key-server.*.address
}
