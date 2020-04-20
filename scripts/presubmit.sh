#!/bin/bash

set -o pipefail

source_dirs="cmd pkg"

echo "🚒 Update Protobufs"
protoc --proto_path=. --go_out=plugins=grpc:. ./pkg/pb/*.proto

echo "🧽 Cleanup Imports"
goimports -w $(echo $source_dirs)

echo "🧹 Format Go code"
find $(echo $source_dirs) -name "*.go" -print0 | xargs -0 gofmt -s -w

echo "🌌 Go mod cleanup"
go mod verify
go mod tidy

echo "🚧 Compile"
go build ./...

echo "🧪 ${X}Test"
go test ./...


