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
SOURCE_DIRS="cmd internal tools"


echo "🌳 Set up environment variables"
export GOMAXPROCS=7
# TODO(sethvargo): configure more


echo "🚒 Verify Protobufs are up to date"
${ROOT}/scripts/dev protoc


echo "📚 Fetch dependencies"
OUT="$(go get -t ./... 2>&1)"
if [ $? -ne 0 ]; then
  echo "✋ Error fetching dependencies"
  echo "\n\n${OUT}\n\n"
  exit 1
fi


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


echo "🐝 Verify spelling"
make spellcheck || {
  echo "✋ Found spelling errors."
  exit 1
}


echo "🔨 Building"
go build ./...


echo "🌌 Verify and tidy module"
OUT="$(go mod verify 2>&1 && go mod tidy 2>&1)"
if [ $? -ne 0 ]; then
  echo "✋ Error validating module"
  echo "\n\n${OUT}\n\n"
  exit 1
fi
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
