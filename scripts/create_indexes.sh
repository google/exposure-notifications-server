#!/bin/bash

pushd "$(dirname "$0")" && gcloud datastore indexes create ../pkg/database/index.yaml; popd
