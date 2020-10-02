---
layout: default
---
# Migration to Google Exposure Notifications Server

If you are running an exposure notifications server (based on this codebase or not)
and want to migrate to another installation that is running this codebase, this document
outlines the process that needds to be done to achieve this.

## Definitions

* For purposes of this document, we'll discuss migration from a __state__ to a __national__
  key server.

* __export-importer__: This server contains a job (`./cmd/export-importer`) that is capable  
  reading signed export files from other servers and importing those TEKs to the
  local storage for re-exporting. We use this in the migration process.

* __Full migration__: The goal of such a migration is to have a state's client
  application write to and read from the national key server and to _decomission_
  the state's key server infrastructure.

* __Read-only migration__: The goal of such a migration is to have a state's client
  application reading from the national exports, but continuing to upload
  to the state server. This could be because of a custom verification protocol.
  In this case, decomissioning the state's key sever infrastructure is a non goal.

## Assumptions

* The application is live, and requires an update in order to switch servers. This update
  could be done via dynamic configuration or new mobile app version.

* There can only be one active export signing key for the application.

* The __state__ key server has a text file index of available zip files that is
  sorted in ascending time order (newest files last). If a state server does
  not have this, it needs to be added before migration can occur.

# Migration Process

## Configure state exports for a second location

The state server must generate export files in a second location (for consuption by the
national sever).

It is recommended to use a separate signing key for these exports than the one already used
for exports destined for clients. This is only a recommendation and is not necessary since
the current signing key will be rotated away from client usage.

## Start importing keys

Configure the __export-importer__ job to import keys from the __state__ server
to the __national__ server.

This requres that the public key portion of the export signing key for the state
is shared with the national server.

These imported keys will start being shared as part of the national export.

## Export mirroring

These next actions need to happen as quickly as possible, and as close together as possible.

The __state__ will rotate it's signing key to be that of the __national__ server
by informing Apple and Google of the change. It's best to request an expedited
rotation if available.

Configure the __state__ server to mirror the __national__ exports on the currently
known client CDN / download location.

### End state

All client apps for the state will now be uploading keys to the __state__ server
and downloading keys from the __national__ server. This gives all users full coverage.

This is a terminal state for the __read-only migration__ option.

## Full migration / application update

The __state__ will now push a client application update that directs clients
to upload to and download from the national server directly.

When this is done, the state server can be decommissioned and the export-importer
job turned down.

If this is being down through an App Store / Play Store application update,
this may take a while and it is up to the state to determine
what an appropriate amount of time to run in this exprort-import-mirror state is.

