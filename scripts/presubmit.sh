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

source_dirs="cmd internal tools"

echo "🚒 Verify Protobufs are up to date"
set +e
$(dirname $0)/gen_protos.sh
git diff *.pb.go| tee /dev/stderr | (! read)
if [ $? -ne 0 ]; then
   echo "✋ Found uncommited changes to generated"
   echo "✋ *.pb.go files. Commit these changes before merging"
   exit 1
fi
set -e

set +e
which goimports >/dev/null 2>&1
if [ $? -ne 0 ]; then
   echo "✋ No 'goimports' found. Please use"
   echo "✋   go install golang.org/x/tools/cmd/goimports"
   echo "✋ to enable import cleanup. Import cleanup skipped."
else
   echo "🧽 Format with goimports"
   goimports -w $(echo $source_dirs)
fi
set -e

echo "🧹 Verify go format"
set -x
diff -u <(echo -n) <(gofmt -d -s .)
set +x

echo "🌌 Go mod verify"
set +e
go mod verify;
set -e

echo "🌌 Go mod tidy"
set -x
go mod tidy;
git diff go.mod | tee /dev/stderr | (! read)
[ -f go.sum ] && git diff go.sum | tee /dev/stderr | (! read)
set +x

echo "🚧 Compile"
go build ./...

echo "🧪 Test"
go test ./... -coverprofile=coverage.out

echo "🧑‍🔬 Test Coverage"
go tool cover -func coverage.out | grep total | awk '{print $NF}'             
