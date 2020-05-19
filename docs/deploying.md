# Exposure Notification Reference Server

## Building and deploying services

This page explains how to build and deploy servers within the Exposure
Notification Reference implementation.

### Before you begin

To build and deploy a service, you need to install and configure the following:

1. Download and install the [Google Cloud SDK](https://cloud.google.com/sdk/install).

    For more information on installation and to set up, see the
    [Cloud SDK Quickstarts](https://cloud.google.com/sdk/docs/quickstarts).

1. Download and install [Go 1.14.0 or newer](https://golang.org/dl/).

    Make sure the `go/bin/` folder is set in your `PATH` environment variable.
    For more information on installing and configuring Go, see
    [Install the Go tools](https://golang.org/doc/install#install).

1. Enable Go modules and install the `ko` container builder and deployment tool:

    ```console
    GO111MODULE=on
    go get github.com/google/ko/cmd/ko
    ```

1. Configure the `ko` tool using the `setup_ko.sh` configuration file:

    ```console
    source setup_ko.sh
    ```

### Building and deploying

To build and deploy a service:

1. Generate a [Google Cloud Repository](https://cloud.google.com/container-registry)
   Docker configuration:

    ```console
    gcloud auth configure-docker
    ```

1. Build and deploy the container using the `ko publish` command from the repository's
   root directory.

    For example, to deploy the exposure key server:

    ```console
    ko publish ./cmd/exposure
    ```

You can find a list of services and their corresponding folders below.

### List of services

The Exposure Notification Reference implementation includes multiple services.
Each service's `main` package is located in the `cmd` directory.

| Service | Folder                | Description |
|---------|-----------------------|-------------|
| exposure key server  | cmd/export | Publishes exposure keys |
| federation | cmd/federation | gRPC federation requests listener |
| federation puller | cmd/federation-pull | Pulls federation results from federation partners |
| exposure server | cmd/exposure |  Stores infection keys |
| exposure cleanup | cmd/cleanup-exposure | Deletes old exposure keys |
| export cleanup | cmd/cleanup-export | Deletes old exported files published by the exposure key export service |

### Deploying using Terraform

You can use Terraform to deploy the reference Exposure Notification on Google
Cloud. These instructions make use of the
[Google Cloud Terraform provider](https://github.com/terraform-providers/terraform-provider-google).

1. Download and install Terraform 0.12.

   To install Terraform using Go, type:

   ```console
   go get github.com/hashicorp/terraform
   ```

   You can find more information on installing Terrarform in the
   [Terraform installation guide](https://www.terraform.io/downloads.html).

1. [Create a Google Cloud project](https://cloud.google.com/resource-manager/docs/creating-managing-projects#creating_a_project)

    Make a note of the project ID, you will be needed later. We recommend
    setting it as an environment variable:

    ```console
    export PROJECT_ID="PROJECT-ID"
    ```

1. **(OPTIONAL)** You can use [Cloud Build triggers](https://cloud.google.com/cloud-build/docs/automating-builds/create-github-app-triggers)
   to automatically re-deploy when the Exposure Notification Server GitHub
   repository is updated. To set up Cloud Build triggers:

   1. Go to the
   [Connect Repository Cloud Console page](https://console.cloud.google.com/cloud-build/triggers/connect)
   and follow the instructions using GitHub as the source code location
   (Cloud Build GitHub App). You must choose a repository on which you have
   administrator permissions.

   1. Make a note of the repository you have chosen. You must set the
   repository name (for example, 'exposure-notification-server') and owner
   (for example 'google') as variables when running the `terraform apply`
   command.

1. Authenticate with Google Cloud using the `gcloud` command-line tool:

   ```console
   gcloud auth login && gcloud auth application-default login
   ```

   This will open two authentication windows in your web browser.

   You may need to `unset GOOGLE_APPLICATION_CREDENTIALS` as it takes precedence
   over gcloud's login settings.

1. Change to this directory and run `terraform init`.  Terraform will
   automatically download the plugins required to execute this code.

1. Run `terraform apply` to start deployment:

   Without Cloud Build Triggers:

   ```console
   terraform apply \
     -var project=$PROJECT_ID
   ```

   With Cloud Build Triggers:

   ```console
   terraform apply \
     -var project=${PROJECT_ID} \
     -var region="us-central-1" \
     -var use_build_triggers=true \
     -var repo_owner=${YOUR_REPO_OWNER} \
     -var repo_name=${YOUR_REPO_NAME}
   ```

Terraform will begin by creating the service accounts and enabling the services
on Google Cloud that are required to run this server.

#### High traffic example deployment

**Important**: This is a production-ready, high traffic sample deployment.
Creating this sample deployment **will generate a substantial bill**. It is
provided as an example, and can be downsized to minimize costs.

```console
  terraform apply \
     -var project=${PROJECT_ID} \
     -var region="us-central-1" \
     -var use_build_triggers=true \
     -var repo_owner=${YOUR_REPO_OWNER} \
     -var repo_name=${YOUR_REPO_NAME} \
     -var cloudsql_tier="db-custom-1-3840" \
     -var cloudsql_disk_size="16"
 ```

<!--
electin vCPU and Postgres size (concurrent connections):

PostgresSql sizing
And pricing
https://cloud.google.com/sql/docs/postgres/create-instance
Combined with connection limits: https://cloud.google.com/sql/docs/quotas#cloud-sql-for-postgresql-connection-limits
Choice for now:

db-custom-8-30720

30 gb which gives 500 concurrent connections

1. Initialize and/or Migrate the DB.

    > **NOTE** In the future this may be handled by terraform

    To migrate the database, you will want to start the
    [Cloud SQL Proxy](https://cloud.google.com/sql/docs/postgres/quickstart-proxy-test#install-proxy)
    and then run the [migrate](https://github.com/golang-migrate/migrate)
    command.

    ```console
    DB_HOST="localhost"
    DB_PORT="1433"
    DB_USER="notification"
    DB_PASSWORD="YOUR-DB-PASSWORD"
    DB_SSLMODE="disable"
    DB_NAME="main"
    DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

    migrate -database ${DB_URL} -path ./migrations up
    ```

--->
