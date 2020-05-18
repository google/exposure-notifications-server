# Starting the exposure notifications server

This is a set of terraform configs which should enable you to bring up a copy of
the exposure notifications server on GCP.  It makes heavy use of the GCP
terraform provider, developed at
https://github.com/terraform-providers/terraform-provider-google.

## Requirements

Go 1.13 or higher.  [Installation guide](https://golang.org/doc/install),
although `apt-get install golang` may be all you need.

Terraform 0.12.  [Installation guide](https://www.terraform.io/downloads.html),
although `go get github.com/hashicorp/terraform` may be all you need.

gcloud.  [Installation guide](https://cloud.google.com/sdk/install), though
`apt-get install google-cloud-sdk` may work.

## Instructions

For full instructions on deploying, view the [deployment docs](../docs/deploying.md)