#!/bin/bash

# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

##
# Runs CI checks for repository.
#
# Parameters
#
# [ARG 1]: Directory for the samples. Default: git/repository.
##

set -ex

go version
date

cd "${1:-git/repository}"

export GO111MODULE=on # Always use modules.
export GOPROXY=https://proxy.golang.org
TIMEOUT=45m

# Tests only run when there are significant changes.
SIGNIFICANT_CHANGES=$(git --no-pager diff --name-only HEAD..master | egrep -v '(\.md$|^\.github)' || true)
# CHANGED_DIRS is the list of significant top-level directories that changed.
# CHANGED_DIRS will be empty when run on master.
CHANGED_DIRS=$(echo $SIGNIFICANT_CHANGES | tr ' ' '\n' | grep "/" | cut -d/ -f1 | sort -u | tr '\n' ' ')

# Override to determine if all go tests should be run.
# Does not include static analysis checks.
RUN_ALL_TESTS="0"
# If this is a nightly test (not a PR), run all tests.
if [ -z ${KOKORO_GITHUB_PULL_REQUEST_NUMBER:-} ]; then
  RUN_ALL_TESTS="1"
# If the change touches a repo-spanning file or directory of significance, run all tests.
elif echo $SIGNIFICANT_CHANGES | tr ' ' '\n' | grep "^go.mod$" || [[ $CHANGED_DIRS =~ "testing" || $CHANGED_DIRS =~ "internal" ]]; then
  RUN_ALL_TESTS="1"
fi

## Static Analysis
# Do the easy stuff before running tests. Fail fast!
set +x

# Fail if a dependency was added without the necessary go.mod/go.sum change
# being part of the commit.
echo "Running 'go.mod/go.sum sync check'..."
set -x
go mod tidy;
git diff go.mod | tee /dev/stderr | (! read)
[ -f go.sum ] && git diff go.sum | tee /dev/stderr | (! read)
set +x

echo "Running 'gofmt compliance check'..."
set -x
diff -u <(echo -n) <(gofmt -d -s .)
set +x

# TODO: Add this back in once a few outstanding failures are handled
# echo "Running 'go vet'..."
# set -x
# go vet ./...
# set +x

pwd
date

# Only run tests if we had detected changes worth running tests
if [ ${RUN_ALL_TESTS} ]; then
  GO_TEST_TARGET="./..."
  GO_TEST_MODULES=$(find . -name go.mod)
  echo "Running all tests"

  set +e

  # Run tests in changed directories that are not in modules.
  exit_code=0
  for i in $GO_TEST_MODULES; do
    mod="$(dirname $i)"
    pushd $mod > /dev/null;
      echo "Running 'go test' in '$mod'..."
      set -x
      2>&1 go test -timeout $TIMEOUT -v ./... | tee sponge_log.log
      cat sponge_log.log | /go/bin/go-junit-report -set-exit-code > sponge_log.xml
      exit_code=$(($exit_code + $?))
      set +x
    popd > /dev/null;
  done
fi

exit $exit_code