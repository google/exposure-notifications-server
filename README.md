# Exposure Notification Reference Server

This repository contains a reference implementation of an exposure notification
server for use in a mobile exposure notification system.

## Overview

The server is responsible for the following functions:

* Accepting the temporary exposure keys of affected users from mobile devices.

* Validating the temporary exposure keys using the device attestation API.

* Storing the temporary exposure keys in a database.

* Periodically generating incremental files that will be downloaded by mobile
  devices to perform the key matching algorithm on the mobile device.

* Sending a public key to devices to digitally sign the incremental files with
  a private key on the device.

* Periodically deleting old temporary exposure keys. After 14 days, or
  configured time period, the exposure keys can no longer be matched to a device.

## Tutorials and reference documentation

You can read tutorials on deploying and using the reference Exposure Notification
Server here:

* [Overview](docs/index.md)
* [Deployment Guide](docs/deploying.md)
* [User Guide](docs/user-guide.md)
* [Reference Documentation](https://godoc.org/github.com/google/exposure-notifications-server)

## Contributing to this project

Contributions to this project are welcomed. For more information about
contributing to this project, see the [contribution guidelines](CONTRIBUTING.md).

