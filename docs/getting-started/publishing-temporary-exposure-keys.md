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

The structure of the API is defined in [pkg/api/v1/exposure_types.go](https://github.com/google/exposure-notifications-server/blob/main/pkg/api/v1/exposure_types.go),
in the `Publish` type. Please see the documentation in the source file for details of the
fields themselves. The 'publish' API is hosted at `/v1/publish` on the `exposure` service. The legacy version (v1alpha1) _may_ also be posted on that same service.

Access to the API depends ont he provided `healthAuthorityID` in the publish request, the
the verification certificate provided in the `verificationPayload` and how things are configured
at the server. Any region metadata assigned to TEKS will be done automatically
at the server. If a TEK is known to be outside of the "home area," then the `traveler` field
should be set to `true`.

The `reportType` field present in TEK exports can ONLY BE SET through a verification certificate.

Here, we point out some non-obvious validation that is applied to the keys. All keys must be valid! If there are any validation errors, the entire batch is rejected.

The following are configurable variables for validating payloads:

| Environment Variable         | Description          | Default |
|------------------------------|----------------------|---------|
| MAX_KEYS_ON_PUBLISH          | Max keys per publish | 30      |
| MAX_SAME_START_INTERVAL_KEYS | Max overlapping keys with same start interval. In practical terms, this means that if you are obtaining TEK history on a mobile device with >= v1.5 of the device API, it will stop the validity of the current day's TEK and issue a new now. Both keys will have the same start interval. |  3  |
| MAX_INTERVAL_AGE_ON_PUBLISH  | Max age. How old keys can be. All provided keys must have a `rollingStartNumber` that is >= to the max age. | 360h (15 days)   |
| MAX_SYMPTOM_ONSET_DAYS       | Max magnitude of days since symptom onset | 21 |

In addition to the above configurations,

* Keys with a future start time (`rollingStartNumber` indicates time > now),
  are rejected.
* Keys that are "still valid" are accepted by the server, but they are embargoed
  until after they key could no longer be replayed usefully. A stall valid key
	is one where the `rollingStartNumber` is in the past, but the
	`rollingStartNumber` + the `rollingPeriod` indicates a future time.
* When using health authority verification certificates
  (__strongly recommended__), the TEK data in the publish request and the
	`hmackey` must be able to be used to calculate the HMAC value as present in
	the certificate.

### The Publish Response

One of the fields of the publish request is the `revisionToken`. The revision token is an encrypted
piece of metadata that must be passed to the server again, if the same device wants to either
upload additional TEKs later or revise the status of TEKs already uploaded. This ensures that
the new upload came from the same device that originated the TEK and that the request go to the
same server only.

The content of the revision token cannot be used to infer that a client ever uploaded keys or
what their diagnosis status is. It is recommended that clients fill this spot in memory
with random data in advance of TEK publish.

## Chaff Requests

It may be possible for a server operator or network observer to glean
information about client's communicating with the server. To minimize this, the
exposure service supports chaff (fake) requests. **Your client should send
regular chaff requests.** To send a chaff request:

1.  Create a real request with a real request body.

1.  Set the `X-Chaff` header on the request.

1.  Receive and **discard** the response. **Do not process the response!**

## Server Access Configuration

In order for your application to publish keys to the server, the server
requires the registration of the Application Name (for Android) or the Bundle ID
(for iOS). This can be done via the admin interface.

To add these entries, we can use the Admin Console Application. For
instructions on how to launch the console, you can view the
[deployment documentation](deploying#configuring-the-server).

Once the Admin Console is launched, you will see a few choices presented.

![](../images/admin_console_landing.png)

First, we need to configure the verification sever piece. Select "Create New Health Authority
Key Configuration"

Create the configuration for the JWT configuration for the verification server's
certificate data, including the issuer, audience, and a human readable name.

![](../images/admin_console_create_new_health_authority.png)

After the initial save, the public key can be added.

Back at the home screen of the admin console, select "Create new Authorized Health Authority"

![](../images/admin_console_add_authorized_application.png)

The "Health Authority ID" field is a string that your mobile app will need to send
on publish requests.

The list of regions provided will be automatically added to all TEKs uploaded via that
health authority ID.

There are two bypass settings that can make development easier for your app developers.

* Revision Token Enforcement Disabled: Must be set to _false_ in production environments. Can be
  set to _true_ for testing
* Health Authority Verification Disabled: Must be set to _false_ in production environments. Can
  be set to _true_ for testing

_Health Authority Certifictes to Accept:_ Check the certificate you configured earlier. This allows
this health authority to trust verification certificates from that verification server.

For more information on diagnosis verification, see:
* Reference server: [exposure-notification-verification-server](https://github.com/google/exposure-notifications-verification-server)
* [verification protocol design](design/verification_protocol.md)
