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

GOFMT_FILES = $(shell go list -f '{{.Dir}}' ./... | grep -v '/pb')
HTML_FILES = $(shell find . -name \*.html)
GO_FILES = $(shell find . -name \*.go)
MD_FILES = $(shell find . -name \*.md)

# lint uses the same linter as CI and tries to report the same results running
# locally. There is a chance that CI detects linter errors that are not found
# locally, but it should be rare.
lint:
	@command -v golangci-lint > /dev/null 2>&1 || (cd $${TMPDIR} && go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.37.1)
	golangci-lint run --config .golangci.yaml
.PHONY: lint

tabcheck:
	@FINDINGS="$$(awk '/\t/ {printf "%s:%s:found tab character\n",FILENAME,FNR}' $(HTML_FILES))"; \
		if [ -n "$${FINDINGS}" ]; then \
			echo "$${FINDINGS}\n\n"; \
			exit 1; \
		fi
.PHONY: tabcheck

test:
	@go test \
		-count=1 \
		-short \
		-timeout=5m \
		./...
.PHONY: test

test-acc:
	@go test \
		-count=1 \
		-race \
		-timeout=10m \
		./... \
		-coverprofile=coverage.out
.PHONY: test-acc

test-coverage:
	@go tool cover -func ./coverage.out | grep total
.PHONY: test-coverage

zapcheck:
	@command -v zapw > /dev/null 2>&1 || (cd $${TMPDIR} && go get github.com/sethvargo/zapw/cmd/zapw)
	@zapw ./...
.PHONY: zapcheck
