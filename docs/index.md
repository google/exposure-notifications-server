# Exposure Notification Reference Server

This repository contains a reference implementation of an exposure notification
server for use in a mobile exposure notification system.

## Tutorials and reference documentation

You can read tutorials on deploying and using the reference Exposure Notification
Server here:

[Deployment guide](deploying.md)  
[Contributor guide](/CONTRIBUTING.md)  
[Server Functional Requirements](server_functional_requirements.md)  
[Server Deployment Options](server_deployment_options.md)
[Reference documentation](https://godoc.org/github.com/google/exposure-notifications-server)  

## Overview

The Exposure Notification Server is responsible for the following functions:

* Accepting the temporary exposure keys of affected users from mobile devices.

* Validating the temporary exposure keys using the
[device attestation API](https://developer.android.com/training/safetynet/attestation).

* Storing the temporary exposure keys in a database.

* Periodically generating incremental files that will be downloaded by mobile
  devices to perform the key-matching algorithm on the mobile device.

* Sending a public key to devices to digitally sign the incremental files with
  a private key on the device.

* Periodically deleting old temporary exposure keys. After 14 days, or
  configured time period, the exposure keys can no longer be matched to a device.

# Issues and Questions

You can open a
[GitHub Issue](https://github.com/google/exposure-notifications-server/issues/new).
or reach out privately by emailing exposure-notifications-feedback@google.com.
Please include as much detail as you can to help in addressing your concern.
