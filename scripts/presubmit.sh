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


OUT="$(go get -t -tags=performance,e2e ./... 2>&1)" || {
  echo "✋ Error fetching dependencies"
  echo "\n\n${OUT}\n\n"
  exit 1
}


OUT="$(go test -i -tags=performance,e2e ./... 2>&1)" || {
  echo "✋ Error fetching test dependencies"
  echo "\n\n${OUT}\n\n"
  exit 1
}


echo "🧹 Verify formatting"
make fmtcheck || {
  echo "✋ Found formatting errors."
  exit 1
}


echo "🐝 Lint"
make staticcheck || {
  echo "✋ Found linter errors."
  exit 1
}


echo "🦶 Verify bodyclose"
make bodyclose || {
  echo "✋ Found unclosed response bodies."
  exit 1
}


echo "🐝 Verify spelling"
make spellcheck || {
  echo "✋ Found spelling errors."
  exit 1
}

echo "👩‍⚖️ Verify copyrights"
make copyrightcheck || {
  echo "✋ Missing copyrights."
  exit 1
}

echo "🔨 Building"
# Compiling *_test.go files with performance tag
go test -c -tags=performance ./internal/performance/...


echo "🌌 Verify and tidy module"
OUT="$(go mod verify 2>&1 && go mod tidy 2>&1)" || {
  echo "✋ Error validating module"
  echo "\n\n${OUT}\n\n"
  exit 1
}
OUT="$(git diff go.mod)"
if [ -n "${OUT}" ]; then
  echo "✋ go.mod is out of sync - run 'go mod tidy'."
  exit 1
fi
OUT="$(git diff go.sum)"
if [ -n "${OUT}" ]; then
  echo "✋ go.sum is out of sync - run 'go mod tidy'."
  exit 1
fi


echo "🧪 Test"
make test-acc
echo "🔬 Test Coverage"
make test-Coverage
