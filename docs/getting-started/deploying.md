---
layout: default
---
<!-- TOC depthFrom:2 depthTo:6 orderedList:false updateOnSave:true withLinks:true -->

- [Before you begin](#before-you-begin)
- [Provisioning infrastructure with Terraform](#provisioning-infrastructure-with-terraform)
- [Running services](#running-services)
  - [Building](#building)
  - [Deploying](#deploying)
  - [Promoting](#promoting)
- [Tracing services](#tracing-services)
- [Running migrations](#running-migrations)
  - [On Google Cloud](#on-google-cloud)
  - [On a custom setup](#on-a-custom-setup)
- [Configuring the server](#configuring-the-server)
- [Configuring components](#configuring-components)
  - [Blob storage](#blob-storage)
  - [Key management](#key-management)
  - [Secrets management](#secrets-management)
  - [Observability](#observability)

<!-- /TOC -->

# Deployment Guide

This page explains how to build and deploy servers within the Exposure
Notification Reference implementation.

The Exposure Notification Reference implementation includes multiple services.
Each service's `main` package is located in the `cmd` directory.

Each service is deployed in the same way, but may accept different configuration
options. Configuration options are specified via environment variables.

| Service          | Folder               | Description |
|------------------|----------------------|-------------|
| cleanup-export   | cmd/cleanup-export   | Deletes old exported files published by the exposure key export service |
| cleanup-exposure | cmd/cleanup-exposure | Deletes old exposure keys |
| export           | cmd/export           | Publishes exposure keys |
| exposure         | cmd/exposure         | Stores infection keys |
| federation-in    | cmd/federation-in    | Pulls federation results from federation partners |
| federation-out   | cmd/federation-out   | gRPC federation requests listener |
| generate         | cmd/generate         | Sample service that generates data |


## Before you begin

To build and deploy the Exposure Notification Key Server services, you need to
install and configure the following:

1. Download and install the [Google Cloud SDK](https://cloud.google.com/sdk/install).

    For more information on installation and to set up, see the
    [Cloud SDK Quickstarts](https://cloud.google.com/sdk/docs/quickstarts).

## Provisioning infrastructure with Terraform

You can use [Terraform](https://www.terraform.io) to provision the initial
infrastructure, database, service accounts, and first deployment of the services
on Cloud Run. **Terraform does not manage the Cloud Run services after their
initial creation!**

See [Deploying with Terraform](https://github.com/google/exposure-notifications-server/blob/main/terraform) for more information.

## Running services

While Terraform does an initial deployment of the services, it does not manage
the Cloud Run services beyond their initial creation. If you make changes to the
code, you need to build, deploy, and promote new services. The general
order of operations is:

1.  **Build** - this is the phase where the code is bundled into a container
    image and pushed to a registry.

1.  **Deploy** - this is the phase where the container image is deployed onto
    Cloud Run, but is not receiving any traffic.

1.  **Promote** - this is the phase where a deployed container image begins
    receiving all or a percentage of traffic.

### Building

Build new services by using the script at `./scripts/build`, specifying the
following values:

-   `PROJECT_ID` (required) - your Google Cloud project ID.

-   `SERVICES` (required) - comma-separated list of names of the services to
    build, or "all" to build all. See the list of services in the table above.

-   `TAG` (optional) - tag to use for the images. If not specified, it uses a
    git-sha-based short tag.

```text
PROJECT_ID="my-project" \
SERVICES="export" \
./scripts/build
```

Expect this process to take 3-5 minutes.

### Deploying

Deploy already-built container using the script at `./scripts/deploy`,
specifying the following values:

-   `PROJECT_ID` (required) - your Google Cloud project ID.

-   `REGION` (required) - region in which to deploy the services.

-   `SERVICES` (required) - comma-separated list of names of the services to
    deploy, or "all" to deploy all. Note, if you specify multiple services, they
    must use the same tag.

-   `TAG` (required) - tag of the deployed image (e.g. abcd1234).

```text
PROJECT_ID="my-project" \
REGION="us-central1" \
SERVICES="export" \
TAG="20200521084829" \
./scripts/deploy
```

Expect this process to take 1-2 minutes.

### Promoting

Promote an already-deployed service to begin receiving production traffic using
the script at `./scripts/promote`, specifying the following values:

-   `PROJECT_ID` (required) - your Google Cloud project ID.

-   `REGION` (required) - region in which to promote the services.

-   `SERVICES` (required) - comma-separated list of names of the services to
    promote, or "all" to deploy all. Note, if you specify multiple services,
    then the revision must be "LATEST".

-   `REVISION` (optional) - revision of the service to promote, usually the
    output of a deployment step. Defaults to "LATEST".

-   `PERCENTAGE` (optional) - percent of traffic to shift to the new revision.
    Defaults to "100".

```text
PROJECT_ID="my-project" \
REGION="us-central1" \
SERVICES="export" \
./scripts/promote
```

Expect this process to take 1-2 minutes.

## Tracing services

To enable distributed tracing, please ensure your environment has these variables

Variable|Values|Comment
---|---|---
OBSERVABILITY_EXPORTER|If unset, no exporting shall be done. Use any of "stackdriver", "prometheus", or "ocagent" otherwise|Note: when using "prometheus" a METRICS_PORT environment variable should also be set
PROJECT_ID|The ProjectID of your associated Google Cloud Platform project on which this application shall be deployed|Required if you use "stackdrver"

## Running migrations

### On Google Cloud

To migrate the production database, use the script in `./scripts/migrate`. This
script triggers a Cloud Build invocation which uses the Cloud SQL Proxy to run
the database migrations and uses the following environment variables:

-   `PROJECT_ID` (required) - your Google Cloud project ID.

-   `DB_CONN` (required) - your Cloud SQL connection name.

-   `DB_PASS_SECRET` (required) - the **reference** to the secret where the
    database password is stored in Secret Manager.

-   `DB_NAME` (default: "main") - the name of the database against which to run
    migrations.

-   `DB_USER` (default: "notification") - the username with which to
    authenticate.

-   `COMMAND` (default: "up") - the migration command to run.

If you created the infrastructure using Terraform, you can get these values by
running `terraform output` from inside the `terraform/` directory:

```text
PROJECT_ID=$(terraform output project)
DB_CONN=$(terraform output db_conn)
DB_PASS_SECRET=$(terraform output db_pass_secret)
```

### On a custom setup

If you did not use the Terraform configurations to provision your server, or if
you are running your own Postgres server,

1.  Download and install the
    [`migrate`](https://github.com/golang-migrate/migrate) tool.

1.  Construct the [database URL](https://github.com/golang-migrate/migrate/tree/main/database/postgres) for your database. This is usually of the format:

    ```text
    postgres://DB_USER:DB_PASSWORD@DB_HOST:DB_PORT/DB_NAME?sslmode=require
    ```

1.  Run the migrate command with this database URL:

    ```text
    migrate \
      -database "YOUR_DB_URL" \
      -path ./migrations \
      up
    ```

## Configuring the server

This repository includes a configuration tool which provides a browser-based
interface for manipulating the database-backed configuration. This admin tool
**does not have authentication / authorization** and **should not be deployed on
the public Internet!**

1.  Export the database connection parameters:

    ```text
    export DB_CONN=...
    export DB_USER=...
    export DB_PASSWORD="secret://..."
    export DB_PORT=...
    export DB_NAME=...
    ```

    If you used Terraform to provision the infrastructure:

    ```text
    cd terraform/
    export DB_CONN=$(terraform output db_conn)
    export DB_USER=$(terraform output db_user)
    export DB_PASSWORD="secret://$(terraform output db_pass_secret)"
    export DB_PORT=5432
    export DB_NAME=$(terraform output db_name)
    cd ../
    ```

1.  Configure the Cloud SQL proxy:

    If you are using Cloud SQL, start the proxy locally:

    ```text
    cloud_sql_proxy -instances=$DB_CONN=tcp:$DB_PORT &
    ```

    And disable SSL verification:

    ```text
    # Cloud SQL uses a local proxy and handles TLS communication automatically
    export DB_SSLMODE=disable
    ```

1.  Start the admin console:

    ```text
    go run ./tools/admin-console
    ```

1.  Open a browser to [localhost:8080](http://localhost:8080/).

    **Remember, you are editing the live configuration of the database!**


## Configuring components

### Blob storage

The blob storage component defines where and how export files are written. The
following configurations are available:

| Name                   | `BLOBSTORE` value      | Description
| ---------------------- | ---------------------- | -----------
| AWS S3                 | `AWS_S3`               | Store data in AWS S3.
| Azure Blobstore        | `AZURE_BLOB_STORAGE`   | Store data in Azure Storage.
| Google Cloud Storage\* | `GOOGLE_CLOUD_STORAGE` | Store data in Google Cloud Storage.
| Filesystem             | `FILESYSTEM`           | Store data on a filesystem.
| Noop                   | `NOOP`                 | No files are written.

\* default

### Key management

The key management component is responsible for signing exports. The following
configurations are available:

| Name               | `KEY_MANAGER` value | Description
| ------------------ | ------------------- | -----------
| AWS KMS            | `AWS_KMS`           | Perform signing using AWS KMS.
| Google Cloud KMS\* | `GOOGLE_CLOUD_KMS`  | Perform signing using Google Cloud KMS.
| HashiCorp Vault    | `HASHICORP_VAULT`   | Perform signing using HashiCorp Vault.
| Noop               | `NOOP`              | No keys are managed.

\* default


### Secrets management

The secrets management component is responsible for acquiring secrets. The
following configurations are available:

| Name                    | `SECRET_MANAGER` value  | Description
| ----------------------- | ----------------------- | -----------
| AWS Secrets Manager     | `AWS_SECRETS_MANAGER`   | Resolve with Secrets Manager.
| Azure Key Vault         | `AZURE_KEY_VAULT`       | Resolve with Key Vault.
| Google Secret Manager\* | `GOOGLE_SECRET_MANAGER` | Resolve with Secret Manager.
| HashiCorp Vault         | `HASHICORP_VAULT`       | Resolve with Vault.
| Noop                    | `NOOP`                  | No secrets are resolved.

\* default

In the interest of performance, secrets are TTL-cached to eliminate the secret
manager from most hot paths. Configure this TTL with `SECRET_CACHE_TTL` which is
expressed as a time duration like "5m" or "15s". The default cache time is 5
minutes and lower values are strongly discouraged.

### Observability

The observability component is responsible for metrics. The following
configurations are available:

| Name                    | `OBSERVABILITY_EXPORTER` value  | Description
| ----------------------- | ------------------------------- | -----------
| OpenCensus Agent        | `OCAGENT`                       | Use OpenCensus.
| Stackdriver\*           | `STACKDRIVER`                   | Use Stackdriver.

\* default
