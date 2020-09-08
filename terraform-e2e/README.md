# terraform-e2e

This directory contains terraform configuration to be used for deploying key servers with terraform, while being able to reuse a project to deploy repeatedly. Should only be used for running e2e tests.

## Prerequisite

Follow steps from [Terrafrom instructions](https://github.com/google/exposure-notifications-server/tree/main/terraform), going through from top until finishing `Instructions` section, then change into `terraform-e2e` directory:

```shell
cd terraform-e2e/
echo "project = \"${PROJECT_ID}\"" >> ./terraform.tfvars
```

## Terraform Apply

### Fresh GCP project

If it's a fresh GCP project, just run `terraform init; terrform apply` under current directory.

### Existing GCP project

If it's a project that had verification server deployed before, run these before `terraform apply`:

```shell
terraform import module.en.google_app_engine_application.app ${PROJECT_ID}
```

#### Troubleshooting: `terraform import` failure

If you ran `terraform import` and see the error of invalid provider configuration like:

```text
Error: Invalid provider configuration

  on ../terraform/main.tf line 15:
  15: provider "google" {

The configuration for
module.en.provider["registry.terraform.io/hashicorp/google"] depends on values
that cannot be determined until apply.
```

This can be mitigated by updating terraform to version equal or greater than 1.13.1

## Terraform Destroy

Terraform destroy always fails while trying to delete db instance. To be reliably successfully destroy, run these ahead:

```shell
db_inst_name="$(terraform output -json 'en' | jq '. | .db_inst_name' | tr -d \")"
gcloud sql instances delete ${db_inst_name} -q --project=${PROJECT_ID}
terraform state rm module.en.google_sql_user.user
terraform state rm module.en.google_sql_ssl_cert.db-cert
```
