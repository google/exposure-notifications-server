# Starting the exposure notification key server

This is a set of Terraform configurations which create the required
infrastructure for an exposure notification key server on Google Cloud. Please note
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

For full instructions on deploying, view the [deployment docs](../docs/getting-started/deploying.md)

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
    $ echo "project = \"${PROJECT_ID}\"" >> ./terraform.tfvars
    ```

1.  (Optional) Enable the data generation job. This is useful for testing
    environments as it provides a consistent flow of exposure data into the
    system.

    ```text
    $ echo 'generate_cron_schedule = "*/15 * * * *"' >> ./terraform.tfvars
    ```

1.  (Optional, but recommended) Create a Cloud Storage bucket for storing remote
    state. This is important if you plan to have multiple people running
    Terraform or collaborating.

    ```text
    $ gsutil mb -p ${PROJECT_ID} gs://${PROJECT_ID}-tf-state
    ```

    Configure Terraform to store state in the bucket:

    ```text
    $ cat <<EOF > ./state.tf
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

### Custom hosts

Using custom hosts (domains) for the services requires a manual step of updating
DNS entries. Run Terraform once and get the `lb_ip` entry. Then, update your DNS
provider to point the A records to that IP address. Give DNS time to propagate
and then re-apply Terraform. DNS must be working for the certificates to
provision.

### Local development and testing example deployment

The default Terraform deployment is a production-ready, high traffic deployment.
For local development and testing, you may want to use a less powerful setup:

```hcl
# terraform/terraform.tfvars
project                  = "..."
cloudsql_tier            = "db-custom-1-3840"
cloudsql_disk_size_gb    = 16
cloudsql_max_connections = 256
```

### Changing Regions

The target cloud region for each resource types are exposed as Terraform
variables in `vars.tf`. Each region or location variable may be changed,
however, they are not necessarily independent. The comments for each variable
make a note of required dependencies and also link to the associated docs page
listing the valid values.

Note that not all resources used by this project are currently available in all
regions, but bringing up infrastructure in different regions needs careful
consideration as geographic location of resources does impact service
performance.

### Developing In Project With Verification Server Also Provisioned by Terraform

**WARNING**: It's **strongly discouraged** to deploy both servers in the same project, do it only if you know what you are doing

When developing in a project with verification server already provisioned, there will be some resources conflict that prevent key server from being provisioned by terraform. It's not a priority to fix since it's not desired to do so in production, instead list potential problems and solutions here if ever needed:

- State file
    - Cause: state files stored in GCS are written into the same GCS bucket `${PROJECT_ID}-tf-state` and will cause conflict with each other
    - Solution: use another bucket or GCS location. i.e. replace `${PROJECT_ID}-tf-state` with `${PROJECT_ID}-key-tf-state` from steps above

- Resources with identical names
    - Cause: `terraform apply` fails when the resource to be provisioned already exists but not in terraform state, so any resource with identical names across two terraform definitions will cause `terraform apply` to fail. So far the known resources with duplicate names are:
        - google_secret_manager_secret.db-secret
        - google_compute_global_address.private_ip_address
        - google_vpc_access_connector.connector
    - Solution: rename these resouces in terraform configurations. i.e. use random string such as [database suffix](https://github.com/google/exposure-notifications-server/blob/025834310ea2bbcb6d05314ff37183bc4c9b91e8/terraform/database.tf#L15)
