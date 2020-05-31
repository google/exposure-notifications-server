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
# Find all non-generated Go files. This works by excluding
# files that start with the official "generated file" header.
# See https://github.com/golang/go/issues/13560#issuecomment-288457920
SOURCE_FILES=($(grep -L -HR "^\/\/ Code generated .* DO NOT EDIT\.$" --include="*.go" ${ROOT}))

echo "🌳 Set up environment variables"
eval $(${ROOT}/scripts/dev init)


echo "🚒 Verify Protobufs are up to date"
${ROOT}/scripts/dev protoc
git diff *.go | tee /dev/stderr | (! read)
if [ $? -ne 0 ]; then
   echo "✋ Found uncommited changes after regenerating Protobufs."
   echo "✋ Commit these changes before merging."
   exit 1
fi


echo "🧽 Verify goimports formattting"
set +e
which goimports >/dev/null 2>&1
if [ $? -ne 0 ]; then
   echo "✋ No 'goimports' found. Please use"
   echo "✋   go get golang.org/x/tools/cmd/goimports"
   echo "✋ to enable import cleanup. Import cleanup skipped."
else
   echo "🧽 Format with goimports"
   goimports -w ${SOURCE_FILES[@]}
   # Check if there were uncommited changes.
   git diff *.go | tee /dev/stderr | (! read)
   if [ $? -ne 0 ]; then
      echo "✋ Found uncommited changes after goimports."
      echo "✋ Commit these changes before merging."
      exit 1
   fi
fi
set -e


echo "🧹 Verify gofmt format"
set +e
gofmt -s -w ${SOURCE_FILES[@]}
git diff *.go | tee /dev/stderr | (! read)
if [ $? -ne 0 ]; then
   echo "✋ Found uncommited changes after gofmt."
   echo "✋ Commit these changes before merging."
   exit 1
fi
set -e


echo "🌌 Go mod verify"
set +e
go mod verify
if [ $? -ne 0 ]; then
   echo "✋ go mod verify failed."
   exit 1
fi
set -e

# Fail if a dependency was added without the necessary go.mod/go.sum change
# being part of the commit.
echo "🌌 Go mod tidy"
set +e
go mod tidy;
git diff go.mod | tee /dev/stderr | (! read)
if [ $? -ne 0 ]; then
   echo "✋ Found uncommited go.mod changes after go mod tidy."
   exit 1
fi
git diff go.sum | tee /dev/stderr | (! read)
if [ $? -ne 0 ]; then
   echo "✋ Found uncommited go.sum changes after go mod tidy."
   exit 1
fi
set -e

echo "🚨 Running 'go vet'..."
go vet ./...


echo "🚧 Compile"
go build ./...


echo "🧪 Test"
go test ./... \
  -coverprofile=coverage.out \
  -count=1 \
  -parallel=20 \
  -timeout=5m


echo "🧑‍🔬 Test Coverage"
go tool cover -func coverage.out | grep total | awk '{print $NF}'
