#!/usr/bin/env bash

set -eEuo pipefail

source_dirs="cmd pkg"

echo "🚒 Update Protobufs"
protoc --proto_path=. --go_out=plugins=grpc:. ./pkg/pb/*.proto

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
