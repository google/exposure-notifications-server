variable "region" {
  default = "us-central1"
}

variable "project" {}

terraform {
  required_providers {
    google      = ">= 3.20.0"
    google-beta = ">= 3.20.0"
    random      = ">= 2.2"
  }
}
