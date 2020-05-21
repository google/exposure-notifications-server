terraform {
  backend "gcs" {
    bucket = "sv-tf-test-16-tf-state"
  }
}
