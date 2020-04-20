#!/bin/sh

protoc --proto_path=. --go_out=plugins=grpc:. ./pkg/pb/*.proto
