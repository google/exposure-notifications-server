# Copyright 2020 the Exposure Notifications Server authors
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

GOFMT_FILES = $(shell go list -f '{{.Dir}}' ./... | grep -v '/pb')
HTML_FILES = $(shell find . -name \*.html)
GO_FILES = $(shell find . -name \*.go)
MD_FILES = $(shell find . -name \*.md)

# diff-check runs git-diff and fails if there are any changes.
diff-check:
	@FINDINGS="$$(git status -s -uall)" ; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "Changed files:\n\n" ; \
			echo "$${FINDINGS}\n\n" ; \
			echo "Diffs:\n\n" ; \
			git diff ; \
			git diff --cached ; \
			exit 1 ; \
		fi
.PHONY: diff-check

generate:
	@go generate ./...
.PHONY: generate

generate-check: generate diff-check
.PHONY: generate-check

# lint uses the same linter as CI and tries to report the same results running
# locally. There is a chance that CI detects linter errors that are not found
# locally, but it should be rare.
lint:
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint
	@golangci-lint run --config .golangci.yaml
.PHONY: lint

# protoc generates the protos
protoc:
	@go install golang.org/x/tools/cmd/goimports@v0.1.12
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2.0
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1
	@protoc --proto_path=. --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. ./internal/pb/*.proto ./internal/pb/federation/*.proto ./internal/pb/export/*.proto
	@goimports -w internal/pb
.PHONY: protoc

# protoc-check re-generates protos and checks if there's a git diff
protoc-check: protoc diff-check
.PHONY: protoc-check

tabcheck:
	@FINDINGS="$$(awk '/\t/ {printf "%s:%s:found tab character\n",FILENAME,FNR}' $(HTML_FILES))" ; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "$${FINDINGS}\n\n" ; \
			exit 1 ; \
		fi
.PHONY: tabcheck

test:
	@go test \
		-shuffle=on \
		-count=1 \
		-short \
		-timeout=5m \
		./...
.PHONY: test

test-acc:
	@go test \
		-shuffle=on \
		-count=1 \
		-race \
		-timeout=10m \
		./... \
		-coverprofile=coverage.out
.PHONY: test-acc

test-coverage:
	@go tool cover -func=./coverage.out
.PHONY: test-coverage

zapcheck:
	@go install github.com/sethvargo/zapw/cmd/zapw
	@zapw ./...
.PHONY: zapcheck
