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

DEV=0

for arg in "$@"
do
    case $arg in
        -d|--dev)
        DEV=1
        shift # Remove --dev from processing
        ;;
    esac
done

set -eEuo pipefail

echo "🌳 Set up environment variables"
echo "# Should Dev: $DEV"
# Docker image build
# instruction(https://github.com/google/mako/blob/v0.2.0/docs/GUIDE.md#quickstore-microservice-as-a-docker-image)
# for mako microservice is currently broken according to mako maintainers. This
# image was craned from gcr.io/knative-tests/test-infra/mako-microservice
export MAKO_IMAGE="gcr.io/oss-prow-build-apollo-server/test/performance/mako-microservice"
export MAKO_PORT="9347"
if [[ -z "${GOOGLE_APPLICATION_CREDENTIALS:-}" ]]; then
  echo "✋ GOOGLE_APPLICATION_CREDENTIALS must be set"
  exit 1
fi

echo "🔨 Start mako microservice"
gcloud auth activate-service-account --key-file=${GOOGLE_APPLICATION_CREDENTIALS}
gcloud auth configure-docker

docker \
  run \
  --rm \
  -v \
  ${GOOGLE_APPLICATION_CREDENTIALS}:/root/adc.json \
  -e \
  "GOOGLE_APPLICATION_CREDENTIALS=/root/adc.json" \
  -p \
  ${MAKO_PORT}:9813 \
  ${MAKO_IMAGE} \
  &

echo "🧪 Test"
if [ $DEV == 1 ]; then
   make performance-dev-test
else
   make performance-test
fi
