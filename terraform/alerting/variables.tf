variable "project" {
  type        = string
  description = "GCP project for key server. Required."
}

variable "notification-email" {
  type        = string
  default     = "nobody@example.com"
  description = "Email address for alerts to go to."
}

terraform {
  required_version = ">= 0.13"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 3.46"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 3.46"
    }
  }
}
