#!/usr/bin/env bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# terrafrom.sh script is used for running e2e test in prow, especially smoke test terrafrom conigurations

set -eEuo pipefail

PROGNAME="$(basename $0)"
ROOT="$(cd "$(dirname "$0")/.." &>/dev/null; pwd -P)"

if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "✋ PROJECT_ID must be set"
  exit 1
fi

if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
  echo "This is local development, authenticate using gcloud"
  echo "gcloud auth login"
  echo "gcloud auth application-default login"
  echo "gcloud auth application-default set-quota-project "${PROJECT_ID}""
else
  echo "GOOGLE_APPLCIATION_CREDENTIALS defined, use it for authentication."
  echo "For local development, run 'unset GOOGLE_APPLCIATION_CREDENTIALS', "
  echo "then rerun this script"
fi

function init() {
  pushd "${ROOT}/terraform-e2e" > /dev/null

  # Preparing for deployment
  echo "project = \"${PROJECT_ID}\"" > ./terraform.tfvars
  # Don't fail if it already exists
  gsutil mb -p ${PROJECT_ID} gs://${PROJECT_ID}-tf-state 2>/dev/null || true
  cat <<EOF > "${ROOT}/terraform-e2e/state.tf"
terraform {
  backend "gcs" {
    bucket = "${PROJECT_ID}-tf-state"
  }
}
EOF

  terraform init --upgrade
  terraform get --update

  popd > /dev/null
}

function deploy() {
  pushd "${ROOT}/terraform-e2e" > /dev/null

  init

  # google_app_engine_application.app is global, cannot be deleted once created,
  # if this project already has it created then terraform apply will fail,
  # importing it can solve this problem
  terraform import module.en.google_app_engine_application.app ${PROJECT_ID} || true

  # Terraform deployment might fail intermittently with certain cloud run 
  # services not up, retry to make it more resilient
  local failed=1
  for i in 1 2 3; do
    if [[ "${failed}" == "0" ]]; then
      break
    fi
    if terraform apply -auto-approve; then
      failed=0
    fi
  done
  popd > /dev/null
  return $failed
}

function destroy() {
  pushd "${ROOT}/terraform-e2e" > /dev/null

  init

  # DB always failed to be destroyed by terraform as it's set to not to destroy,
  # so delete it manually
  local db_inst_name
  # Fetching databases from previous terraform deployment output is not always reliable,
  # especially when previous terraform deployment failed. So grepping from terraform state instead.
  db_inst_name="$(tf state show module.en.google_sql_database_instance.db-inst | grep -Eo 'en-verification-[a-zA-Z0-9]+' | uniq)"
  if [[ -n "${db_inst_name}" ]]; then
    echo "Delete DB ${db_inst_name}"
    gcloud sql instances delete ${db_inst_name} -q --project=${PROJECT_ID}
  fi
  # Clean up states after manual DB delete
  terraform state rm module.en.google_sql_user.user || true
  terraform state rm module.en.google_sql_ssl_cert.db-cert || true
  terraform destroy -auto-approve
  popd > /dev/null
}

function smoke() {
  # Best effort destroy before applying
  destroy || true
  trap "destroy || true" EXIT
  deploy
}

# help prints help.
function help() {
  echo 1>&2 "Usage: ${PROGNAME} <command>"
  echo 1>&2 ""
  echo 1>&2 "Commands:"
  echo 1>&2 "  init         init terraform"
  echo 1>&2 "  deploy       deploy server"
  echo 1>&2 "  destroy      destroy server"
  echo 1>&2 "  smoke        deploy then destroy server"
}

case "${1:-}" in
  "" | "help" | "-h" | "--help" )
    help
    ;;

  "init" | "deploy" | "destroy" | "smoke" )
    $1
    ;;

  *)
    help
    exit 1
    ;;
esac
