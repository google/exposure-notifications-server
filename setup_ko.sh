#!/bin/sh

export PROJECT_ID=$(gcloud config get-value core/project)
export KO_DOCKER_REPO="us.gcr.io/${PROJECT_ID}"
export DOCKER_REPO_OVERRIDE="us.gcr.io/${PROJECT_ID}"
