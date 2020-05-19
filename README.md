# Exposure Notification Reference Server

[COVID‑19 Exposure Notifications API](https://www.google.com/covid19/exposurenotifications/)

In our continued effort to help governments and health authorities during the
COVID-19 pandemic, we have authored an open source reference implementation of
an Exposure Notifications server.

The server reference in this repository implements the Exposure Notifications
API and provides reference code for working with Android and iOS apps that
are built by public health authorities. The reference server source code is
available on GitHub and can be deployed on any infrastructure or cloud
provider selected by a public health authority.

Our hope is by making this privacy-preserving server implementation available
to health authorities, we can enable their developers to use the open source code
to get started quickly. 

## Overview

The server is responsible for the following functions:

* Accepting the temporary exposure keys of affected users from mobile devices.

* Validating the temporary exposure keys using the device attestation API.

* Storing the temporary exposure keys in a database.

* Periodically generating incremental files that will be downloaded by mobile
  devices to perform the key matching algorithm on the mobile device.

* Sending a public key to devices, and digitally signing the incremental files with
  a private key.

* Periodically deleting old temporary exposure keys. After 14 days, or
  configured time period, the exposure keys can no longer be matched to a device.

## Tutorials and reference documentation

You can read tutorials on deploying and using the reference Exposure Notification
Server here:

* [Overview](docs/index.md)
* [Deployment Guide](docs/deploying.md)
* [Reference Documentation](https://pkg.go.dev/mod/github.com/google/exposure-notifications-server)
* [Server Functional Requirements](docs/server_functional_requirements.md)
* [Server Deployment Options](docs/server_deployment_options.md)

## Issues and Questions

You can open a
[GitHub Issue](https://github.com/google/exposure-notifications-server/issues/new).
Please be sure to include as much detail as you can to help aid in addressing
your concern. If you wish to reach out privately, you can send an e-mail
exposure-notifications-feedback@google.com.

## Contributing to this project

Contributions to this project are welcomed. For more information about
contributing to this project, see the [contribution guidelines](CONTRIBUTING.md).

