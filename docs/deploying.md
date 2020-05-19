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

    ```
    GO111MODULE=on
    go get github.com/google/ko/cmd/ko
    ```

1. Configure the `ko` tool using the `setup_ko.sh` configuration file:

    ```
    source setup_ko.sh
    ```

### Building and deploying

To build and deploy a service:

1. Generate a [Google Cloud Repository](https://cloud.google.com/container-registry)
   Docker configuration:

    ```
    gcloud auth configure-docker
    ```

1. Build and deploy the container using the `ko publish` command from the repository's
   root directory.

    For example, to deploy the exposure key server:

    ```
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

### Deploying Using Terraform configs

The included terraform configs should enable you to bring up a copy of
the exposure notifications server on GCP.  It makes heavy use of the GCP
terraform provider, developed at
https://github.com/terraform-providers/terraform-provider-google.

1. Download and install Terraform 0.12.  [Installation guide](https://www.terraform.io/downloads.html),
although `go get github.com/hashicorp/terraform` may be all you need.

1.  Create a GCP project.
    [Instructions](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    Enable a billing account for this project, and remember its project ID (the
    unique, unchangeable string that you will be asked for during creation).
    
    ```text
    $ export PROJECT_ID="<value-from-above>"
     ```

1.  (OPTIONAL) Decide whether or not to use cloud build triggers. If you do, 
    every push to master on the GitHub repo containing the exposure server code
    will trigger a new deployment. To enable this:

    1. Visit https://console.cloud.google.com/cloud-build/triggers/connect and follow the instructions to connect as a Cloud Build GitHub App. You must choose a repository that you have admin permissions on.

    1. Remember which repo you used. You will need to set the repo owner (e.g. 'google') and name (e.g. 'exposure-notifications-server') as variables in the `terraform apply`

1.  Authenticate to gcloud with:

    ```text
    $ gcloud auth login && gcloud auth application-default login
    ```

    This will open two authentication windows in your web browser.

    >  **NOTE** You may need to `unset GOOGLE_APPLICATION_CREDENTIALS` as it
    >  takes precedence over the gcloud login settings.

1.  Change to this directory and run `terraform init`.  Terraform will
    automatically download the plugins required to execute this code.

1.  Execute Terraform:

    Without Cloud Build Triggers:

    ```text
    $ terraform apply \
        -var project=$PROJECT_ID
    ```

    With Cloud Build Triggers:

    ```text
    $ terraform apply \
        -var project=${PROJECT_ID} \
        -var region="us-central-1" \
        -var use_build_triggers=true \
        -var repo_owner=${YOUR_REPO_OWNER} \
        -var repo_name=${YOUR_REPO_NAME}
    ```

Terraform will begin by creating the service accounts and enabling the services
on GCP which are required to run this server.

> NOTE: This configuration assumes production scale. The scale of this means
> a substantial billed amount. You can downsize this to save on costs
> For example you can set other vars in terraform apply to smaller values.
> ```
>  $ terraform apply \
>       -var project=${PROJECT_ID} \
>       -var region="us-central-1" \
>       -var use_build_triggers=true \
>       -var repo_owner=${YOUR_REPO_OWNER} \
>       -var repo_name=${YOUR_REPO_NAME} \
>       -var cloudsql_tier="db-custom-1-3840" \
>       -var cloudsql_disk_size="16"
> ```


electin vCPU and Postgres size (concurrent connections):

PostgresSql sizing
And pricing
https://cloud.google.com/sql/docs/postgres/create-instance
Combined with connection limits: https://cloud.google.com/sql/docs/quotas#cloud-sql-for-postgresql-connection-limits
Choice for now:

db-custom-8-30720


30 gb which gives 500 concurrent connections




1.  Initialize and/or Migrate the DB.

    > **NOTE** In the future this may be handled by terraform
    
    To migrate the database, you will want to start the
    [Cloud SQL Proxy](https://cloud.google.com/sql/docs/postgres/quickstart-proxy-test#install-proxy)
    and then run the [migrate](https://github.com/golang-migrate/migrate)
    command.
    
    ```text
    $ DB_HOST="localhost"
    $ DB_PORT="1433"
    $ DB_USER="notification"
    $ DB_PASSWORD="YOUR-DB-PASSWORD"
    $ DB_SSLMODE="disable"
    $ DB_NAME="main"
    $ DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

    $ migrate -database ${DB_URL} -path ./migrations up
    ```

