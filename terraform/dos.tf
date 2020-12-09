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

# This file contains the Cloud Armor configs.

resource "google_compute_security_policy" "cloud-armor" {
  name = "cloud-armor-protection"
  rule {
    action      = "deny(403)"
    description = "XSS protection"
    match {
      expr { expression = "evaluatePreconfiguredExpr('xss-stable')" }
    }
    preview  = false
    priority = 10
  }
  rule {
    action      = "deny(403)"
    description = "SQL Injection protection"
    match {
      expr { expression = "evaluatePreconfiguredExpr('sqli-stable')" }
    }
    preview  = false
    priority = 20
  }
  rule {
    action      = "deny(403)"
    description = "Remote Code Execution protection"
    match {
      expr { expression = "evaluatePreconfiguredExpr('rce-stable')" }
    }
    preview  = false
    priority = 30
  }
  // Recommended action when we want to protect the service during incident:
  // 1. See if there's any rule below can be used as is. Use that.
  // 2. Otherwise check
  //    https://cloud.google.com/armor/docs/rules-language-reference and see if
  //    there's other attributes that you can use.
  // 3. Run `terraform apply` with "preview=true" first.
  // 4. Go to "Monitoring - Dashboards" and choose "Network Security Policies".
  //    Check the "Previewd Requests" graph at the bottom and see how many
  //    traffic would be blocked if "preview=false". Ensure the rule doesn't
  //    block all traffic by accident. If it does block too many traffics, go
  //    back and review the expression.
  // 5. Run `terraform apply` again with "preview=false".
  //
  // Block certain User-Agent
  // ========================
  // rule {
  //   action      = "deny(403)"
  //   description = "Block User-Agent FOO"
  //   match {
  //     expr {
  //       expression = <<-EOT
  //         has(request.headers['user-agent']) &&
  //         request.headers.['user-agent'].matches('FOO')
  //       EOT
  //     }
  //   }
  //   preview  = true
  //   priority = 40
  // }
  //
  // Block IP ranges
  // ===============
  // rule {
  //   action      = "deny(403)"
  //   description = "Block IP Range FOO"
  //   match {
  //     expr {
  //       expression = <<-EOT
  //         inIPRange(origin.ip, '1.2.3.0/24')
  //       EOT
  //     }
  //   }
  //   preview  = true
  //   priority = 50
  // }
  //
  // Block access to a certain URL path
  // ==================================
  // rule {
  //   action      = "deny(403)"
  //   description = "Block path FOO"
  //   match {
  //     expr {
  //       expression = <<-EOT
  //         request.path.matches('FOO')
  //       EOT
  //     }
  //   }
  //   preview  = true
  //   priority = 50
  // }
  rule {
    action      = "allow"
    description = "Default rule, higher priority overrides it"
    match {
      config { src_ip_ranges = ["*"] }
      versioned_expr = "SRC_IPS_V1"
    }
    preview  = false
    priority = 2147483647
  }
}
