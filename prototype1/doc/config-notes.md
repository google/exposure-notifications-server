# Notes on various one time setup.

```
gsutil mb gs://apollo-server-273118-deploy
```

## Prototype1 

Setup cloud function:
   * Authentication: Allow unauthenticated invocations
   * Source code: ZIP from Cloud Storage
   * Runtime: Node.js 8
   * Cloud Storage location: apollo-server-273118-deploy/prototype1.zip
   * Function to execute: api

URL: https://us-central1-apollo-server-273118.cloudfunctions.net/prototype1

### Cloud Bigtable

Setup .cbrtrc
```
project = apollo-server-273118
instance = prototype1
```

Create the tables:
```
cbt createtable infections
cbt createfamily infections properties
cbt createtable verifications
cbt createfamily verifications properties
cbt createtable verifiers
cbt createfamily verifiers properties
cbt createtable prefix_index
cbt createfamily prefix_index properties
```
