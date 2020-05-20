# Exposure Notification Server

## Verifying iOS devices using DeviceCheck

**Important: Using the DeviceCheck API requires a valid and active Apple iOS Developer
subscription.**

Apple uses the [DeviceCheck API](https://developer.apple.com/documentation/devicecheck)
to verify device integrity. As part of the DeviceCheck verification process, the
server receives a token that is generated on the device to verify the iOS
device.

When the server receives the `device_token` from an iOS device, it needs to
communicate with Apple's servers to verify the authenticity of the token. This
request requires authentication and authorization. Authentication is provided
via a signed JSON Web Token (JWT) and authorization is provided based on the
grants given to the signing key. You must provide the signing keys that the
server uses to construct and sign the JWT.

Each developer, called a "team", requires a different key for verification. If
you are running a multi-tenant server that supports iOS apps from multiple
developers, you will need to acquire and register DeviceCheck for each team.

## Finding your Team ID

To find your Team ID:

1. [Create or sign in to your developer account](https://developer.apple.com/account/#/membership).

1. Find the value for **Team ID** in the table on this page. It is usually a
   10 character alpha-numeric ID such as `ABCDE1FGHI`.

## Generating the DeviceCheck private key

In order for the server to communicate with Apple's servers, you will need to
create an Apple authenticate key with `DeviceCheck` enabled.

1. [Sign in to your developer account](https://developer.apple.com/account/#/membership).

1. Choose **Keys** in the side menu.

1. Click **Add** to create a new key.

1. Enter a name for the key. This is a descriptive name and will not be used
   in later steps.

1. Check the box next to **DeviceCheck** to enable using the DeviceCheck with
   this key.

1. Click **Continue**.

1. On the confirmation page, do the following:

   1. Save the DeviceCheck Key ID value to a secure location - this is usually
      a 10 character alpha-numeric ID such as `ABCDE1FGHI`.

   1. Download the DeviceCheck Private Key file (with a `.p8` file extension)
      and save it in a secure location. **This file is a private key, you should
      treat it like a password.**

## Configuring the server to use DeviceCheck

To enable and configure Apple's DeviceCheck on the server, you must specify the
following information for each iOS application your server supports:

- **Team ID** - developer ID, used as the `iss` in the JWT.

- **Key ID** - key ID, used as the `kid` in the JWT.

- **Private Key** - private key (`.p8`), used to sign the JWT.

## Sharing the DeviceCheck key with server operators

You need to share these values with your server operator. If your server is
operated by a third party, the preferred way to share these secrets is using
[Secret Manager](https://cloud.google.com/secret-manager/docs/). The secret
resides in a Google Cloud project that you control, and you can revoke access
to the secret at any time.

### Prerequisites

- A [Google Cloud account](https://console.cloud.google.com/freetrial) and a
  [project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

  When you create a project, keep a note of the project ID, you will need it in
  this tutorial.

- The [`gcloud`](https://cloud.google.com/sdk/install) command-line tool.

- Your Team ID, Key ID, and Private Key in .p8 format. These values come from
  the Apple Developer Portal using the instructions above.

- The server operator's service account email. Your server operator will share
  this value. This will be of the form `name@project.iam.gserviceaccount.com`.

- A communication channel between yourself and your server operator, such as a
  shared chat room, email, or a tele/video conference bridge.

### Sharing the DeviceCheck Private Key

To share the DeviceCheck Private Key with a server operator:

1. If you have not already done so, authenticate the gcloud CLI:

   ```text
   gcloud auth login && gcloud auth application-default login
   ```

   This will open two browser windows and ask you to authenticate with your
   Google account. Use the same account that owns the Google Cloud project.

1. Enable the Secret Manager service on your Google Cloud project:

   ```text
   gcloud services enable secretmanager.googleapis.com \
      --project "${PROJECT_ID}"
   ```

1. Create a secret and upload the private key into the secret:

   ```text
   gcloud secrets create "devicecheck-key" \
      --project "${PROJECT_ID}" \
      --replication-policy "automatic" \
      --data-file PATH-TO-KEY-FILE
   ```

1. Grant the server's service account the ability to access the secret:

   ```text
   gcloud secrets add-iam-policy-binding "devicecheck-key" \
      --project "${PROJECT_ID}" \
      --role "roles/secretmanager.secretAccessor" \
      --member "[EMAIL]" # <-- replace with the value from the server operator
   ```

1. Get the secret resource ID, for sharing with the server operator:

   ```text
   gcloud secrets describe "1" \
      --project "${PROJECT_ID}" \
      --secret "devicecheck-key" \
      --format "value(name)"
   ```

    You should see something like the following:

   ```text
   projects/123456789/secrets/devicecheck-key/versions/1
   ```

1. Using the existing communication channel with your server operator, share
   your:

   - Team ID
   - Key ID
   - Private Key **Resource ID**
