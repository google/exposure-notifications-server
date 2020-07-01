---
layout: default
---
# Exposure Notification Reference Key Server

[COVID‑19 Exposure Notifications API](https://www.google.com/covid19/exposurenotifications/)

In our continued effort to help governments and health authorities during the
COVID-19 pandemic, we have authored an open source reference implementation of
an Exposure Notification Key Server.

The server reference in this repository implements the Exposure Notifications
API and provides reference code for working with Android and iOS apps that
are built by public health authorities. The reference server source code is
available on GitHub and can be deployed on any infrastructure or cloud
provider selected by a public health authority.

Our hope is by making this privacy-preserving server implementation available
to health authorities, we can enable their developers to use the open source code
to get started quickly.

## Overview

The Exposure Notification Key Server is responsible for the following functions:

* Accepting the temporary exposure keys of affected users from mobile devices.

* Validating the temporary exposure keys using a [configurable third-party
  verification service](design/verification_protocol.md).

* Storing the temporary exposure keys in a database.

* Periodically generating incremental files that will be downloaded by mobile
  devices to perform the key-matching algorithm on the mobile device.

* Sending a public key to devices, and digitally signing the incremental files with a private key.

* Periodically deleting old temporary exposure keys. After 14 days, or
  configured time period, the exposure keys can no longer be matched to a device.

## Tutorials and reference documentation

You can read tutorials on deploying and using the reference Exposure Notification
Key Server here:

* [Deployment guide](getting-started/deploying.md)
* [Contributor guide](https://github.com/google/exposure-notifications-server/blob/main/CONTRIBUTING.md)
* [Server Functional Requirements](server_functional_requirements.md)
* [Server Deployment Options](server_deployment_options.md)
* [Reference documentation](https://pkg.go.dev/mod/github.com/google/exposure-notifications-server)
* [API Definitions](https://github.com/google/exposure-notifications-server/tree/main/pkg/api/v1alpha1)

## Issues and Questions

You can open a
[GitHub Issue](https://github.com/google/exposure-notifications-server/issues/new).
or reach out privately by emailing exposure-notifications-feedback@google.com.
Please include as much detail as you can to help in addressing your concern.
