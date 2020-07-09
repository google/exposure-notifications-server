---
layout: default
---

# Temporary Exposure Key (TEK) Publishing Guide

This page explains how to configure the server and application to allow
publishing keys from mobile applications to the server.

## Prerequisites

{% include getting-started-prerequisites.md %}

## What is a Temporary Exposure Key?
Temporary Exposure Keys, or TEKs, are anonymous keys shared between mobile
devices to determine if two devices were sufficiently nearby to be considered
"exposed" to one another. When an application user confirms they have been
exposed, their keys are shared to the server in order for applications to
download and [determine if other users interacted with any of the now exposed
keys](https://blog.google/documents/69/Exposure_Notification_-_Cryptography_Specification_v1.2.1.pdf).

## The Publish API Call

TEKs are published by sending the appropriate JSON document in the body of
an HTTP POST request to the `exposure` server.

The structure of the API is defined in [pkg/api/v1alpha1/exposure_types.go](https://github.com/google/exposure-notifications-server/blob/main/pkg/api/v1alpha1/exposure_types.go),
in the `Publish` type. Please see the documentation in the source file
for details of the fields themselves.

Here, we point out some non-obvious validation that is applied to the keys.

* All keys must be valid! If there are any validation errors, the entire batch
  is rejected.
* Max keys per publish: Default is `20` and can be adjusted with the
  `MAX_KEYS_ON_PUBLISH` environment variable.
* Max overlapping keys with same start interval: Default is `3` and can be
  adjusted with the `MAX_SAME_START_INTERVAL_KEYS` environment variable.
	In practical terms, this means that if you are obtaining TEK history on a
	mobile device with >= v1.5 of the device API, it will stop the validity
	of the current day's TEK and issue a new now. Both keys will have the same
	start iterval.
* Max age: How old keys can be. The default is `360h` (15 days) and can be
  adjusted with the `MAX_INTERVAL_AGE_ON_PUBLISH`. All provided keys must have
	a `rollingStartNumber` that is >= to the max age.
* Keys with a future start time (`rollingStartNumber` indicates time > now),
  are rejected.
* Keys that are "sill valid" are accepted by the server, but they are embargoed
  until after they key could no longer be replayed usefully. A stall valid key
	is one where the `rollingStartNumber` is in the past, but the
	`rollingStartNumber` + the `rollingPeriod` indicates a future time.
* When using health authority verification certificates
  (__strongly recommended__), the TEK data in the publish request and the
	`hmackey` must be able to be used to calculate the HMAC value as present in
	the certificate.

## Server Access Configuration

In order for your application to publish keys to the server, the server
requires the registration of the Application Name (for Android) or the Bundle ID
(for iOS). This can be done via the admin interface.

To add these entries, we can use the Admin Console Application. For
instructions on how to launch the console, you can view the
[deployment documentation](deploying#configuring-the-server).

Once the Admin Console is launched, you will see a few choices presented.

![](../images/admin_console_landing.png)

In the Admin Console, select "Create new Authorized Application"

![](../images/admin_console_add_authorized_application.png)

You are required to provide the App Package Name and/or Bundle ID that will be
publishing to the server. You can optionally configure a limited set of regions
that will be accepted. This region is included as part of the publish payload.
While it is expected this is likely a Country, State, or Provice Code, the
string is opaque to the server. You can use this to delineate any boundary you
find useful. These regions used on upload are used at export time. Each Export
Configuration can be comprised of one or more region codes. This provides
flexibility in how the keys are grouped and distributed to mobile apps.

Optionally you can also disable Health Authority Verification. This is not
recommended as it would allow wide-access publishing of temporary exposure
keys. To configure a health authority, you can select the configuration page
from the main landing page of the Admin Console.

![](../images/admin_console_create_new_health_authority.png)

For more information on creating a verification server, see the design for a
[configurable third-party verification service](design/verification_protocol.md)
