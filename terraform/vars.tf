variable "region" {
  type    = string
  default = "us-central1"
}

variable "appengine_location" {
  type    = string
  default = "us-central"
}

variable "project" {
  type = string
}

variable "use_build_triggers" {
  type    = bool
  default = false
}

variable "repo_owner" {
  type    = string
  default = "google"
}

variable "repo_name" {
  type    = string
  default = "exposure-notifications-server"
}

variable "cloudsql_tier" {
  type    = string
  default = "db-custom-32-122880"

  description = "Size of the CloudSQL tier. Set to db-custom-1-3840 or a smaller instance for local dev."
}

terraform {
  required_providers {
    google      = "~> 3.20"
    google-beta = "~> 3.20"
    null        = "~> 2.1"
    random      = "~> 2.2"
  }
}
