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

export PROJECT_ID=$(gcloud config get-value core/project)

# environment variables for running the services
export DATASTORE_PROJECT_ID=$PROJECT_ID

# local application credentials - you need to get your own credentials
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/local/sa.json"

# To use Secret Manager, you need to grant Secret Manager > Secret Manager Accessor to the credentials in the above sa.json file.
# https://pantheon.corp.google.com/iam-admin/iam
#
# If using a test project, you need to create a test key (one time) in your project:
#
#  $ echo -n "<some random string>" | gcloud secrets create safetynetapi --replication-policy=automatic --data-file=-
#
export SAFETYNET_API_KEY="projects/$PROJECT_ID/secrets/safetynetapi/versions/latest"
export SAFETYNET_URL="https://www.googleapis.com/androidcheck/v1/attestations/verify?key="

# wipeout variables
export TTL_DURATION="14d"

# Set up environment for local postgres database; see pkg/database/schema.sql for the schema to use.
# More configuration available in pkg/database/connection.go
export DB_HOST=localhost
export DB_PORT=5432
export DB_DBNAME=apollo
export DB_USER=apollo
export DB_PASSWORD=mypassword
export DB_SSLMODE=disable


if [ ! -f "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
    echo "$GOOGLE_APPLICATION_CREDENTIALS does not exist. \
Use https://console.cloud.google.com/iam-admin/serviceaccounts/create?project=$PROJECT_ID to create a service account \
with Datastore->Cloud Datastore User, then create a key and download the JSON file and store it at \
$GOOGLE_APPLICATION_CREDENTIALS"
    exit -1
fi

echo "Project ID:    $PROJECT_ID"
echo "Credentials:   $GOOGLE_APPLICATION_CREDENTIALS"
echo "Database:      $DB_HOST:$DB_PORT"
echo "Database user: $DB_USER"
