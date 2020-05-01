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

echo "🚒 Update Protobufs"
$(dirname $0)/gen_protos.sh

set +e
which goimports >/dev/null 2>&1
if [ $? -ne 0 ]; then
   echo "✋ No 'goimports' found. Please use"
   echo "✋   go install golang.org/x/tools/cmd/goimports"
   echo "✋ to enable import cleanup. Import cleanup skipped."
else
   echo "🧽 Format"
   goimports -w $(echo $source_dirs)
fi
set -e

echo "🧹 Format Go code"
find $(echo $source_dirs) -name "*.go" -print0 | xargs -0 gofmt -s -w

echo "🌌 Go mod cleanup"
go mod verify
go mod tidy

echo "🚧 Compile"
go build ./...

echo "🧪 Test"
go test ./...
