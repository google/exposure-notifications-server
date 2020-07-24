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

PROGNAME="$(basename $0)"

if [[ -z "${PROJECT_ID:-}" ]]; then
  echo "PROJECT_ID must be set" >&2
  exit 1
fi
gcloud config set project ${PROJECT_ID}

DB_ZONE="${DB_ZONE:-us-central}"
DB_INSTANCE_NAME="${DB_INSTANCE_NAME:-en-$(openssl rand -hex 12)}"
DB_NAME="${DB_NAME:-$(openssl rand -hex 12)}"
DB_USER="${DB_USER:-main}"
DB_PASSWORD="${DB_PASSWORD:-$(openssl rand -hex 64)}"
SQL_TIER="${SQL_TIER:-db-custom-1-3840}"
DB_VERSION="${DB_VERSION:-POSTGRES_11}"
DB_STORAGE_SIZE="${DB_STORAGE_SIZE:-16}"
VPC_NETWORK="${VPC_NETWORK:-projects/${PROJECT_ID}/global/networks/default}"

function export_private_ip() {
  ip_address="$(gcloud \
    sql \
    instances \
    describe \
    ${DB_INSTANCE_NAME} \
    --project=${PROJECT_ID} \
    --format='value(ipAddresses[1].ipAddress)')"

  if [[ -z "${ip_address}" ]]; then
    echo "Failed to get ip address of db instance"
    exit 1
  fi
  echo "export DB_HOST=\"${ip_address}\""
}

function export_env_var() {
  echo "export DB_INSTANCE_NAME=\"${DB_INSTANCE_NAME}\""
  echo "export DB_ZONE=\"${DB_ZONE}\""
  echo "export DB_NAME=\"${DB_NAME}\""
  echo "export DB_USER=\"${DB_USER}\""
  echo "export DB_PASSWORD=\"${DB_PASSWORD}\""
}

function db_instance_exist() {
  output="$(gcloud \
    sql \
    instances \
    list \
    --filter="name=${DB_INSTANCE_NAME}" \
    --project=${PROJECT_ID})"
  if [[ -n "${output}" ]]; then
    return 0 # Exist
  fi
  return 1
}

function db_user_exist() {
  output="$(gcloud \
    sql \
    users \
    list \
    --filter="name=${DB_USER}" \
    --instance="${DB_INSTANCE_NAME}" \
    --project=${PROJECT_ID})"
  if [[ -n "${output}" ]]; then
    return 0 # Exist
  fi
  return 1
}

function db_exist() {
  output="$(gcloud \
    sql \
    databases \
    list \
    --filter="name=${DB_NAME}" \
    --instance="${DB_INSTANCE_NAME}" \
    --project=${PROJECT_ID})"
  if [[ -n "${output}" ]]; then
    return 0 # Exist
  fi
  return 1
}

function setup() {
  if ! db_instance_exist; then
    echo "🔨 Creating ${DB_INSTANCE_NAME} in ${PROJECT_ID}" >&2
    gcloud \
      beta \
      sql \
      instances \
      create \
      ${DB_INSTANCE_NAME} \
      --database-version=${DB_VERSION} \
      --tier=${SQL_TIER} \
      --storage-size=${DB_STORAGE_SIZE} \
      --network=${VPC_NETWORK} \
      --region=${DB_ZONE} \
      --project=${PROJECT_ID}
  fi

  if ! db_user_exist; then
    echo "🔨 Creating user ${DB_USER} in ${DB_INSTANCE_NAME}" >&2
    gcloud \
      sql \
      users \
      create \
      ${DB_USER} \
      --instance=${DB_INSTANCE_NAME} \
      --password=${DB_PASSWORD} \
      --project=${PROJECT_ID}
  fi

  if ! db_exist; then
    echo "🔨 Creating database ${DB_NAME} in ${DB_INSTANCE_NAME}" >&2
    gcloud \
      sql \
      databases \
      create \
      ${DB_NAME} \
      --instance=${DB_INSTANCE_NAME} \
      --project=${PROJECT_ID}
  fi

  export_env_var
  export_private_ip
}

function teardown() {
  echo "tear down"
  if [[ "${TEARDOWN_DB_INSTANCE:-}" == "1" ]]; then
    echo "🔨 Delete ${DB_INSTANCE_NAME} in ${PROJECT_ID}"
    gcloud \
      sql \
      instances \
      delete \
      ${DB_INSTANCE_NAME} \
      --quiet \
      --project=${PROJECT_ID}
  else
    if [[ "${TEARDOWN_DB_USER:-}" == "1" ]]; then
      echo "🔨 Delete user ${DB_USER} in ${DB_INSTANCE_NAME}"
      gcloud \
        sql \
        users \
        delete \
        ${DB_USER} \
        --instance=${DB_INSTANCE_NAME} \
        --quiet \
        --project=${PROJECT_ID}
    fi
    if [[ "${TEARDOWN_DB:-}" == "1" ]]; then
      echo "🔨 Delete DB ${DB_NAME} in ${DB_INSTANCE_NAME}"
      gcloud \
        sql \
        databases \
        delete \
        ${DB_NAME} \
        --instance=${DB_INSTANCE_NAME} \
        --quiet \
        --project=${PROJECT_ID}
    fi
  fi
}

# help prints help.
function help() {
  echo 1>&2 "Usage: ${PROGNAME} <command>"
  echo 1>&2 ""
  echo 1>&2 "Commands:"
  echo 1>&2 "  setup         creating database"
  echo 1>&2 "  teardown      delete database"
}

SUBCOMMAND="${1:-}"
case "${SUBCOMMAND}" in
  "" | "help" | "-h" | "--help" )
    help
    ;;

  "setup" | "teardown" | "export_env_var" | "export_private_ip" )
    shift
    ${SUBCOMMAND} "$@"
    ;;

  *)
    help
    exit 1
    ;;
esac
