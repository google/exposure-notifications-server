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

# TODO(chaodaiG): make sure comment this out before merging
export PROJECT_ID="chao-en-e2e-exp"

echo "🌳 Set up environment variables"
if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "✋ PROJECT_ID must be set"
  exit 1
fi

# Best effort destroy before applying
./scripts/terraform.sh destroy 2>/dev/null || true
./scripts/terraform.sh deploy

if [[ -z "${DB_CONN:-}" ]]; then # Allow custom database
  echo "🔨 Provision servers"
  pushd terraform
  # TODO(chaodaiG): terraform init; terraform apply; trap "terraform destroy"
  export DB_CONN="$(terraform output 'db_conn')"
  export DB_NAME="$(terraform output 'db_name')"
  export DB_USER="$(terraform output 'db_user')"
  export DB_PASSWORD="secret://$(terraform output db_pass_secret)"
  export DB_SSLMODE=disable
  popd
fi


# TODO(chaodaiG): cloud_sql_proxy hasn't been installed yet, uncomment these once terraform confirmed to work

wget https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O /usr/bin/cloud_sql_proxy
chmod +x /usr/bin/cloud_sql_proxy

echo "🔨 Run cloud sql proxy"
which cloud_sql_proxy || {
  echo "✋ Download cloud_sql_proxy from https://cloud.google.com/sql/docs/mysql/connect-admin-proxy#install"
  exit 1
}
cloud_sql_proxy -instances=${DB_CONN}=tcp:5432 &
CLOUD_SQL_PROXY_PID=$!
trap "kill $CLOUD_SQL_PROXY_PID || true" EXIT


echo "🧪 Test"
go test \
  -count=1 \
  -race \
  -timeout=10m \
  ./internal/integration

# Don't fail even if destroy failed
./scripts/terraform.sh destroy || true
