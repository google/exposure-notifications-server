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

To add these entries, follow the instructions for
[configuring the server](/getting-started/deploying#configuring-the-server).
Via the admin-console, you can add the App Package Name and App Bundle ID to
be allowed to publish exposure keys.

While not required, if a verification server is required, additional
configuration will be required. Please see the design for a
[configurable third-party verification service](design/verification_protocol.md)