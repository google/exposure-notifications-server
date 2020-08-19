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
ALL_SERVICES="cleanup-export,cleanup-exposure,export,exposure,federationin,federationout,generate,key-rotation"

START_TIME=$(date)

export PROJECT_ID="chao-en-e2e-exp"
export REGION="us-central1"
export TAG="081320-$(openssl rand -hex 12)"
export SERVICES="all"

./scripts/build
./scripts/deploy
./scripts/promote

not_ready=0
IFS=',' read -ra SERVICES_ARR <<< "${SERVICES}"
for SERVICE in "${SERVICES_ARR[@]}"; do
  revision_exist="$( gcloud \
    run \
    revisions \
    list \
    --platform=managed \
    --service=${SERVICE} \
    --project=${PROJECT_ID} \
    --region=${REGION} | \
    grep '${TAG}' || true)"
  [[ -z "${revision_exist}" ]] && { not_ready=1; break; }
  echo "${SERVICE} is ready"
done

echo "Start time: ${START_TIME}"
echo "$(date)"

exit $not_ready
