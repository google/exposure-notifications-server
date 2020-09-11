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

ROOT="$(cd "$(dirname "$0")/.." &>/dev/null; pwd -P)"

function main() {
  # PROW_JOB_ID is an env var set by prow, use project for prow when it's in prow
  if [[ -z "${PROJECT_ID:-}" && -n "${PROW_JOB_ID:-}" ]]; then
      PROJECT_ID="$(boskos_acquire)"
      trap "boskosctl_wrapper release --name \"${PROJECT_ID}\" --target-state dirty >&2" EXIT
      export PROJECT_ID
  fi

  ${ROOT}/scripts/terraform.sh smoke
}


# Functions below are prow helpers

# Wrapper for running all boskos commands, similar to struct functions in Go
# JOB_NAME is defined in prow
function boskosctl_wrapper() {
    boskosctl --server-url "http://boskos.test-pods.svc.cluster.local." --owner-name "${JOB_NAME}" "${@}"
}

# Acquire GCP project from Boskos, the manager of projects pool
# Return the project name being acquired
function boskos_acquire() {
  local resource
  local resource_name
  echo "Try to acquire project from boskos" >&2
  resource="$( boskosctl_wrapper acquire --type key-smoke-project --state free --target-state busy --timeout 10m )"
  resource_name="$( jq .name <<<"${resource}" | tr -d \" )"
  echo "Acquired project from boskos: ${resource_name}" >&2
  # Send a heartbeat in the background to keep the lease while using the resource.
  boskosctl_wrapper heartbeat --resource "${resource}" >/dev/null 2>&1 &
  echo "${resource_name}"
}


main
