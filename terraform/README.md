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

1. Decide whether or not to use cloud build triggers. If you do, every push to master on the GitHub repo containing
the exposure server code will trigger a new deployment. To enable this:

  1. Visit https://console.cloud.google.com/cloud-build/triggers/connect and follow the instructions to connect as a Cloud Build GitHub App. You must choose a repository that you have admin permissions on.

  1. Remember which repo you used. You will need to set the repo owner (e.g. 'google') and name (e.g. 'exposure-notifications-server') as variables in the `terraform apply`

1. Log in to gcloud using `gcloud auth application-default login`.

1. Change to this directory and run `terraform init`.  Terraform will
automatically download the plugins required to execute this code.

1. Run `terraform apply -var project=$YOUR_PROJECT_ID_FROM_STEP_1 [-var use_build_triggers=true -var repo_owner=$YOUR_REPO_OWNER -var repo_name=$YOUR_REPO_NAME]`.

Terraform will begin by creating the service accounts and enabling the services
on GCP which are required to run this server.

It will then create the database and user and apply the DB schema, and run the assorted binaries with everything hooked up.
