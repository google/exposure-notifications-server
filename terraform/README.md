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

1. Create a GCP project.
[Instructions](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
Enable a billing account for this project, and remember its project ID (the
unique, unchangeable string that you will be asked for during creation).

1. Log in to gcloud using `gcloud auth login`.

1. Change to this directory and run `terraform init`.  Terraform will
automatically download the plugins required to execute this code.

1. Run `terraform apply -var project=$YOUR_PROJECT_ID_FROM_STEP_1`.

Terraform will begin by creating the service accounts and enabling the services
on GCP which are required to run this server.

It will then create the database and user and apply the DB schema.

TODO(ndmckinley): Next it will trigger a build of this project using Google Cloud Build, and
run the binaries.  In this way, code changes made to this repository can be
rolled out using terraform apply.
