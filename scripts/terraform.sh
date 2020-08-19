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

set -eEuo pipefail

ROOT="$(cd "$(dirname "$0")/.." &>/dev/null; pwd -P)"

if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "✋ PROJECT_ID must be set"
fi

if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
  if [[ ! -f "${HOME}/.config/gcloud/application_default_credentials.json" ]]; then
    echo "This is local development, authenticate using gcloud"
    gcloud auth login && gcloud auth application-default login
  fi
else
  echo "GOOGLE_APPLCIATION_CREDENTIALS defined, use it for authentication."
  echo "For local development, run 'unset GOOGLE_APPLCIATION_CREDENTIALS', "
  echo "then rerun this script"
fi

function deploy() {
  pushd "${ROOT}/terraform" > /dev/null
  
  # Preparing for deployment
  echo "project = \"${PROJECT_ID}\"" >> ./terraform.tfvars
  gsutil mb -p ${PROJECT_ID} gs://${PROJECT_ID}-tf-state 2>/dev/null || true
  cat <<EOF > ./state.tf
terraform {
  backend "gcs" {
    bucket = "${PROJECT_ID}-tf-state"
  }
}
EOF

  # google_app_engine_application.app is global, cannot be deleted once created,
  # if this project already has it created then terraform apply will fail,
  # importing it can solve this problem
  terraform init
  terraform import google_app_engine_application.app ${PROJECT_ID} || true
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
  pushd "${ROOT}/terraform" > /dev/null
  local db_inst_name
  db_inst_name="$(terraform output 'db_inst_name')"
  # DB often failed to be destroyed by terraform due to "used by other process",
  # so delete it manually
  gcloud sql instances delete ${db_inst_name} -q --project=chao-en-e2e-exp
  # Clean up states after manual DB delete
  terraform state rm google_sql_user.user
  terraform state rm google_sql_ssl_cert.db-cert
  terraform destroy -auto-approve
  popd > /dev/null
}

# help prints help.
function help() {
  echo 1>&2 "Usage: ${PROGNAME} <command>"
  echo 1>&2 ""
  echo 1>&2 "Commands:"
  echo 1>&2 "  deploy       deploy server"
  echo 1>&2 "  destroy      destroy server"
}

case "$1" in
  "" | "help" | "-h" | "--help" )
    help
    ;;

  "deploy" | "destroy" )
    $1
    ;;

  *)
    help
    exit 1
    ;;
esac
