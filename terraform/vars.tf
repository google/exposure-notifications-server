variable "region" {
  type = string
  default = "us-central1"
}

variable "project" {
  type = string
}

variable "use_build_triggers" {
  type = bool
  default = false
}

variable "repo_owner" {
  type = string
  default = "google"
}

variable "repo_name" {
  type = string
  default = "exposure-notifications-server"
}

terraform {
  required_providers {
    google      = "~> 3.20"
    google-beta = "~> 3.20"
    null        = "~> 2.1"
    random      = "~> 2.2"
  }
}
