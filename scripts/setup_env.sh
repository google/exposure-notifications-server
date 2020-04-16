#!/bin/sh

export PROJECT_ID=$(gcloud config get-value core/project)

# environment variables for running the services
export DATASTORE_PROJECT_ID=$PROJECT_ID
export DIAGNOSIS_KMS_KEY="projects/$PROJECT_ID/locations/us/keyRings/us-db-keys/cryptoKeys/diagnosisKeys"

# local application credentials - you need to get your own credentials
export GOOGLE_APPLICATION_CREDENTIALS="$(pwd)/local/sa.json"
