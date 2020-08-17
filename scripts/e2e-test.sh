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

  pushd "${ROOT}/terraform-e2e"
  DB_CONN="$(terraform output -json 'en' | jq '. | .db_conn' | tr -d \")"
  DB_NAME="$(terraform output -json 'en' | jq '. | .db_name' | tr -d \")"
  DB_USER="$(terraform output -json 'en' | jq '. | .db_user' | tr -d \")"
  DB_PASSWORD="secret://$(terraform output -json 'en' | jq '. | .db_password' | tr -d \")"
  EXPOSURE_URL="$(terraform output -json 'en' | jq '. | .exposure_url' | tr -d \")"
  popd
  export DB_CONN
  export DB_NAME
  export DB_USER
  export DB_PASSWORD
  export EXPOSURE_URL

  export DB_SSLMODE=disable

  which cloud_sql_proxy 1>/dev/null 2>&1 || {
    wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O /usr/bin/cloud_sql_proxy
    chmod +x /usr/bin/cloud_sql_proxy
  }
  cloud_sql_proxy -instances=${DB_CONN}=tcp:5432 &
  go test \
    -count=1 \
    -timeout=30m \
    -v \
    ./internal/e2e
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
