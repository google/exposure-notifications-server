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

# Note: other environment variables may be set by the test infrastructure. See:
# https://github.com/GoogleCloudPlatform/oss-test-infra/tree/master/prow/prowjobs/google/exposure-notifications-server.
echo "🌳 Set up environment variables"
export GOMAXPROCS=7


# Authenticate and pull private Docker repos.
if [ -n "${CI:-}" ]; then
  gcloud --quiet auth configure-docker us-docker.pkg.dev
fi
if [ -n "${CI_POSTGRES_IMAGE:-}" ]; then
  docker pull --quiet "${CI_POSTGRES_IMAGE}"
fi
if [ -n "${CI_REDIS_IMAGE:-}" ]; then
  docker pull --quiet "${CI_REDIS_IMAGE}"
fi


OUT="$(go get -t -tags=all ./... 2>&1)" || {
  echo "✋ Error fetching dependencies"
  echo "\n\n${OUT}\n\n"
  exit 1
}


OUT="$(go test -i -tags=all ./... 2>&1)" || {
  echo "✋ Error fetching test dependencies"
  echo "\n\n${OUT}\n\n"
  exit 1
}


echo "🧪 Test"
make test-acc

echo "🔬 Test Coverage"
make test-coverage
