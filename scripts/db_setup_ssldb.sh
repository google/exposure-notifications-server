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

## TO use this SSL connection, your DB host has to be via IP and not via socket.

export PROJECT_ID=$(gcloud config get-value core/project)

unset DB_PASSWORD

# To use Secret Manager, you need to grant Secret Manager > Secret Manager Accessor to the credentials in the above sa.json file.
# https://console.cloud.google.com/iam-admin/iam
#
# Using the cloud console - create a SSL authorization on your CloudSQL database
# and store the values generated in 3 secretes in Secret Manager:
# https://console.cloud.google.com/security/secret-manager
#
# dbServerCA, dbClientCert, and dbClientKey
# It is recommended to locate these scretes in the same cloud region as your
# server + database.
export DB_SSLMODE="verify-ca"
export DB_SSLROOTCERT_SECRET="projects/$PROJECT_ID/secrets/dbServerCA/versions/latest"
export DB_SSLCERT_SECRET="projects/$PROJECT_ID/secrets/dbClientCert/versions/latest"
export DB_SSLKEY_SECRET="projects/$PROJECT_ID/secrets/dbClientKey/versions/latest"
