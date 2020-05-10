#!/bin/sh

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

# Run migrations on the database

command -v migrate >/dev/null 2>&1 || {
    echo >&2 "Migrate command not found. Have you installed golang-migrate?";
    echo >&2 "https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md#installation";
    exit 1;
}

if [[ -z "$1" ]]; then
    echo "Expected the type of migration to run as an argument (up, down, goto, force, version)."
    exit 1
fi

# DB URL Parameters are set in setup_env.sh
export POSTGRESQL_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

# Force uses a second arg
migrate -database ${POSTGRESQL_URL} -path migrations $1 $2
