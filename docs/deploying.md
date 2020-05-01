# Exposure Notification Reference Server

TODO(llatif): produce a list of servers that can be deployed.

## Building and deploying servers

To build and deploy a server, you will need to install the `ko` container
builder tool and the [Google Cloud SDK](https://cloud.google.com/sdk/).

1. Download and install the [Google Cloud SDK](https://cloud.google.com/sdk/install).
For more information on installation and set up, see the
[Cloud SDK Quickstarts](https://cloud.google.com/sdk/docs/quickstarts).

1. Install the `ko` container builder and deployment tool:

    ```
    GO111MODULE=on
    go get github.com/google/ko/cmd/ko
    ```

1. Configure the `ko` tool using the `setup_ko.sh` configuration file in this
   repository:

    ```
    source setup_ko.sh
    ```

1. Generate a [Google Cloud Repository](https://cloud.google.com/container-registry)
   Docker configuration:

    ```
    gcloud auth configure-docker
    ```

1. Build and deploy the container using the `ko publish` command.

    For example, to deploy the infection server:

    ```
    ko publish ./cmd/infection
    ```