# Prototype 1

An attempt to capture basic requirements, worrying only a little bit about scale.

## Tools Install

[Firebase CLI Install](https://firebase.google.com/docs/cli)
```
curl -sL https://firebase.tools | bash
firebase login
```

[Cloud SDK Install](https://cloud.google.com/sdk/docs)
```
./google-cloud-sdk/install.sh
./google-cloud-sdk/bin/gcloud init
```

[Cloud Bigtable Install](https://cloud.google.com/bigtable/docs/installing-cloud-sdk)
```
gcloud components update
gcloud components install beta cbt
gcloud config set project apollo-server-273118
```

[gsutil Install](https://cloud.google.com/storage/docs/gsutil_install)
```
curl https://sdk.cloud.google.com | bash
```

## Directories

### firebase/

A web app intended for health status verification.

### cloud-functions/

A Cloud Functions deployment to support the backend for scanning and verification.
