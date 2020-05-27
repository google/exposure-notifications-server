# Starting the exposure notifications server

This is a set of Terraform configurations which create the required
infrastructure for an exposure notifications server on Google Cloud. Please note
that **Terraform is only used for the initial deployment and provisioning of
underlying infrastructure!** It is not used for continuous delivery or
continuous deployment.

## Requirements

- Terraform 0.12. [Installation guide](https://www.terraform.io/downloads.html)

- gcloud. [Installation guide](https://cloud.google.com/sdk/install)

    Note: Make sure you **unset** `GOOGLE_APPLICATION_CREDENTIALS` in your
    environment:

    ```text
    unset GOOGLE_APPLICATION_CREDENTIALS
    ```

## Instructions

For full instructions on deploying, view the [deployment docs](../docs/deploying.md)

1.  Create a GCP project.
    [Instructions](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
    Enable a billing account for this project, and note its project ID (the
    unique, unchangeable string that you will be asked for during creation):

    ```text
    $ export PROJECT_ID="<value-from-above>"
    ```

1.  Authenticate to gcloud with:

    ```text
    $ gcloud auth login && gcloud auth application-default login
    ```

    This will open two authentication windows in your web browser.

1.  Change into the `terraform/` directory. All future commands are run from the
    `terraform/` directory:

    ```text
    $ cd terraform/
    ```

1.  Save the project ID as a Terraform variable:

    ```text
    $ echo "project = ${PROJECT_ID}" >> ./terraform.tfvars
    ```

1.  (Optional) Enable the data generation job. This is useful for testing
    environments as it provides a consistent flow of exposure data into the
    system.

    ```text
    $ echo "create_generate_service = true" >> ./terraform.tfvars
    ```

1.  (Optional, but recommended) Create a Cloud Storage bucket for storing remote
    state. This is important if you plan to have multiple people running
    Terraform or collaborating.

    ```text
    $ gsutil mb -p ${PROJECT_ID} gs://${PROJECT_ID}-tf-state
    ```

    Configurre Terraform to store state in the bucket:

    ```text
    $ cat <<EOF > ./terraform/state.tf
    terraform {
      backend "gcs" {
        bucket = "${PROJECT_ID}-tf-state"
      }
    }
    EOF
    ```

1.  Run `terraform init`. Terraform will automatically download the plugins
    required to execute this code. You only need to do this once per machine.

    ```text
    $ terraform init
    ```

1.  Execute Terraform:

    ```text
    $ terraform apply
    ```

Terraform will create the required infrastructure including the database,
service accounts, storage bucket, keys, and secrets. **As a one-time
operation**, Terraform will also migrate the database schema and build/deploy
the initial set of services on Cloud Run. Terraform does not manage the
lifecycle of those resources beyond their initial creation.

### Local development and testing example deployment

The default Terraform deployment is a production-ready, high traffic deployment.
For local development and testing, you may want to use a less powerful setup:

```hcl
# terraform/terraform.tfvars
project               = "..."
cloudsql_tier         = "db-custom-1-3840"
cloudsql_disk_size_gb = "16"
```
