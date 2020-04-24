# Contact Tracing Reference Server

This repository contains a reference implementation of an exposure notification
server intended to be used as a backend for a mobile device exposure
notification solution.

The server is responsible for the following functions:
- Accepting the temporary exposure keys of affected users from mobile devices,
  validating those keys via device attestation APIs, and storing those keys in
  a database.
- Periodically, generating incremental files for download by client devices for
  performing the key matching algorithm (all key matching happens on the mobile
  devices, not on the server). These incremental files must be digitally signed
  with a private key. The corresponding public key is pushed to mobile devices
  via separate configuration.
- Periodically deleting old temporary exposure keys. After 14 days (or
  configurable different time periods) they can no longer be matched to
  devices.
