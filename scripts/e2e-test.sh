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

# TODO: use project pool for this
# PROW_JOB_ID is an env var set by prow, use project for prow when it's in prow
if [[ -z "${PROJECT_ID:-}" && -n "${PROW_JOB_ID:-}" ]]; then
    echo "Using project for prow since"
    export PROJECT_ID=serious-physics-287516
fi

${ROOT}/scripts/terraform.sh smoke
