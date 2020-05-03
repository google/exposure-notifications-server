# Exposure Notification Reference Server

## Building and deploying services

This page explains how to build and deploy servers within the Exposure
Notification Reference implementation.

### Before you begin

To build and deploy a service, you need to install and configure:

1. Download and install the [Google Cloud SDK](https://cloud.google.com/sdk/install).

    For more information on installation and set up, see the
    [Cloud SDK Quickstarts](https://cloud.google.com/sdk/docs/quickstarts).

1. Download and install [Go 1.14.0 or newer](https://golang.org/dl/).

    Make sure the `go/bin/` folder is set in your `PATH` environment variable.
    For more information on installing and configuring Go, see
    [Install the Go tools](https://golang.org/doc/install#install).

1. Enable Go modules and install the `ko` container builder and deployment tool:

    ```
    GO111MODULE=on
    go get github.com/google/ko/cmd/ko
    ```

1. Configure the `ko` tool using the `setup_ko.sh` configuration file:

    ```
    source setup_ko.sh
    ```

### Building and deploying

To build and deploy a service:

1. Generate a [Google Cloud Repository](https://cloud.google.com/container-registry)
   Docker configuration:

    ```
    gcloud auth configure-docker
    ```

1. Build and deploy the container using the `ko publish` command from the repository's
   root directory.

    For example, to deploy the exposure key server:

    ```
    ko publish ./cmd/exposure
    ```

You can find a list of services and their corresponding folders below.

## List of services

The Exposure Notification Reference implementation includes multiple services.
Each service's `main` package is located in the `cmd` directory.

| Service | Folder                | Description |
|---------|-----------------------|-------------|
<<<<<<< HEAD
| exposure key server  | cmd/export | Publishes exposure keys |
| federation | cmd/federation | gRPC federation requests listener |
| federation puller | cmd/federation-pull | Pulls federation results from federation partners |
| exposure server | cmd/exposure |  Stores infection keys |
| exposure cleanup | cmd/cleanup-export | Deletes old exposure keys |
| export cleanup | cmd/cleanup-infection | Deletes old exported files published by the exposure key export service |
=======
| exposure key export  | cmd/export | Publishes exposure keys |
| federation | cmd/federation | gRPC federation requests listener |
| federation puller | cmd/federation-pull | Pulls federation results from federation partners |
| infection server | cmd/infection |  Stores infection keys |
| exposure wipeout | cmd/wipeout-export | Deletes old exposure keys |
| infection wipeout | cmd/wipeout-infection | Deletes old infection keys |
>>>>>>> 044f586... Ran through the deploying instructions and got stuck as my PATH didn't have the go/bin directory. Should be mentioned here so that folks unfamiliar with go are not tripped up.
