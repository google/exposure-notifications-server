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

# e2e-test.sh script is the entrypoint for running e2e tests in prow

set -eEuo pipefail

PROGNAME="$(basename $0)"
ROOT="$(cd "$(dirname "$0")/.." &>/dev/null; pwd -P)"


function smoke() {
  # PROW_JOB_ID is an env var set by prow, use project for prow when it's in prow
  if [[ -z "${PROJECT_ID:-}" && -n "${PROW_JOB_ID:-}" ]]; then
      PROJECT_ID="$(boskos_acquire key-smoke-project)"
      trap "boskosctl_wrapper release --name \"${PROJECT_ID}\" --target-state dirty >&2" EXIT
      export PROJECT_ID
  fi

  ${ROOT}/scripts/terraform.sh smoke
}


function incremental() {
  # PROW_JOB_ID is an env var set by prow, use project for prow when it's in prow
  if [[ -z "${PROJECT_ID:-}" && -n "${PROW_JOB_ID:-}" ]]; then
      PROJECT_ID="$(boskos_acquire key-e2e-project)"
      trap "boskosctl_wrapper release --name \"${PROJECT_ID}\" --target-state dirty >&2" EXIT
      export PROJECT_ID
  fi

  ${ROOT}/scripts/terraform.sh init

  ${ROOT}/scripts/build
  ${ROOT}/scripts/deploy
  ${ROOT}/scripts/promote

  export_terraform_output db_conn DB_CONN
  export_terraform_output db_name DB_NAME
  export_terraform_output db_user DB_USER
  export_terraform_output db_password DB_PASSWORD
  export_terraform_output exposure_url EXPOSURE_URL
  
  export DB_PASSWORD="secret://${DB_PASSWORD}"
  export DB_SSLMODE=disable

  run_e2e_test
}

function run_e2e_test() {
  which cloud_sql_proxy 1>/dev/null 2>&1 || {
    wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O /usr/bin/cloud_sql_proxy
    chmod +x /usr/bin/cloud_sql_proxy
  }
  cloud_sql_proxy -instances=${DB_CONN}=tcp:5432 &
  last_thread_pid=$!
  trap "kill ${last_thread_pid} || true" EXIT

  make e2e-test
}

# Export en module output
# $1 ... terraform output variable name
# $2 ... exported variable name
function export_terraform_output() {
  local output
  pushd "${ROOT}/terraform-e2e" >/dev/null 2>&1
  output="$(terraform output -json "en" | jq ". | .${1}" | tr -d \")"
  popd >/dev/null 2>&1
  eval "export ${2}=${output}"
}


# Functions below are prow helpers

# Wrapper for running all boskos commands, similar to struct functions in Go
# JOB_NAME is defined in prow
function boskosctl_wrapper() {
    boskosctl --server-url "http://boskos.test-pods.svc.cluster.local." --owner-name "${JOB_NAME}" "${@}"
}

# Acquire GCP project from Boskos, the manager of projects pool
# Return the project name being acquired
# $1 ... boskos project type
function boskos_acquire() {
  local resource
  local resource_name
  echo "Try to acquire project from boskos" >&2
  resource="$( boskosctl_wrapper acquire --type $1 --state free --target-state busy --timeout 10m )"
  resource_name="$( jq .name <<<"${resource}" | tr -d \" )"
  echo "Acquired project from boskos: ${resource_name}" >&2
  # Send a heartbeat in the background to keep the lease while using the resource.
  boskosctl_wrapper heartbeat --resource "${resource}" >/dev/null 2>&1 &
  echo "${resource_name}"
}


# help prints help.
function help() {
  echo 1>&2 "Usage: ${PROGNAME} <command>"
  echo 1>&2 ""
  echo 1>&2 "Commands:"
  echo 1>&2 "  smoke        terraform smoke test"
  echo 1>&2 "  incremental  incremental e2e test"
}

ACTION="${1:-}"
case "${ACTION}" in
  "" | "help" | "-h" | "--help" )
    help
    ;;

  "smoke" | "incremental" )
    "${ACTION}"
    ;;

  *)
    help
    exit 1
    ;;
esac
