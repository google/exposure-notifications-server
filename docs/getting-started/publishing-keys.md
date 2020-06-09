---
layout: default
---

# Key Publishing Guide

This page explains how to configure the server and application to allow
publishing keys.

## Prerequisites

If you haven't started developing a mobile app, please take a look at the
[Android Exposure Notifications Reference App](https://github.com/google/exposure-notifications-android)

You will need to
[complete the necessary steps](https://support.google.com/googleplay/android-developer/contact/expo_notif_api)
to use the exposure notifcations API. You should also ensure that you have
taken the steps to comply with the
[Terms of Service](https://blog.google/documents/72/Exposure_Notifications_Service_Additional_Terms.pdf).
For instance, your app may not request any of the following permissions:
  * ACCESS_FINE_LOCATION
  * ACCESS_COARSE_LOCATION
  * ACCESS_BACKGROUND_LOCATION
  * BLUETOOTH_ADMIN

## Publishing Keys

In order for your application to publish keys to the server, the server
requires the addition of the Application Name (for Android) and the Bundle ID
(for iOS). This can be done via the admin interface.

To add these entries, we can use the Admin Console Application. For
instructions on how to launch the console, you can view the
[deployment documentation](/getting-started/deploying#configuring-the-server).

Once the Admin Console is launched, you will see a few choices presented.

![](../images/admin_console_landing.png)

In the Admin Console, select "Create new Authorized Application"

![](../images/admin_console_add_authorized_application.png)

You are required to provide the App Package Name that will be publishing to
the server. You can optionally configure a limited set of regions that will be
accepted. This region is included as part of the publish payload. While it is
expected this is likely a Country, State, or Provice Code, the string is opaque
to the server. You can use this to delineate any boundary you find useful. 

Optionally you can also disable Health Authority Verification. This is not
recommended as it would allow wide-access publishing of temporary exposure
keys. To configure a health authority, you can select the configuration page
from the main landing page of the Admin Console.

![](../images/admin_console_create_new_health_authority.png)

For more information on creating a verification server, see the design for a
[configurable third-party verification service](design/verification_protocol.md)
