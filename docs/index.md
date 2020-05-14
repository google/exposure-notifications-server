# Exposure Notification Reference Server

https://www.google.com/covid19/exposurenotifications/

In our continued effort to help governments and health authorities during the COVID-19 pandemic, we have authored an open source reference implementation of an Exposure Notifications server. The server reference design implements the Exposure Notifications API and provides reference code for working with both Android and iOS apps that are built by public health authorities. The reference server source code is available on GitHub and can be deployed on any modern infrastructure or cloud provider selected by a public health authority. 

The reference server implementation accepts, validates, and stores temporary exposure keys from verified mobile devices. It also periodically generates and signs incremental files that will later be downloaded by clients to perform the on-device key matching algorithm. Our hope is by making this privacy-preserving server implementation available to health authorities we can enable their developers to use the open source code to get started quickly. 

This repository contains a reference implementation of an exposure notification
server for use in a mobile exposure notification system.

## Overview

The server is responsible for the following functions:

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

## Tutorials and reference documentation

You can read tutorials on deploying and using the reference Exposure Notification
Server here:

* [Deployment guide](deploying.md)
* [User guide](user-guide.md)
* [Contributor guide](/CONTRIBUTING.md)
* [Reference documentation](https://godoc.org/github.com/google/exposure-notifications-server)
* [Server Functional Requirements](server_functional_requirements.md)
* [Server Deployment Options](server_deployment_options.md)

# Issues and Questions

You can open a
[GitHub Issue](https://github.com/google/exposure-notifications-server/issues/new).
Please be sure to include as much detail as you can to help aid in addressing
your concern. If you wish to reach out privately, you can send an e-mail
exposure-notifications-feedback@google.com.
