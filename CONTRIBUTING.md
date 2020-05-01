# Exposure Server Components - contribution guidelines

Contributions to this project are welcomed. We request that you
read through the guidelines before getting started.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement (CLA). You (or your employer) retain the copyright to your
contribution; this simply gives us permission to use and redistribute your
contributions as part of the project. Head over to
<https://cla.developers.google.com/> to see your current agreements on file or
to sign a new one.

You generally only need to submit a CLA once, so if you've already submitted one
(even if it was for a different project), you probably don't need to do it
again.

## Community guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google/conduct/).

## Contribution

### Code reviews

All submissions will be reviewed before merging. Submissions are reviewed using
[GitHub pull requests](https://help.github.com/articles/about-pull-requests/).

### Presubmits

You should run the presubmit checks before committing changes. The presubmit script
is located at `scripts/presubmit.sh`. You can add a prepush hook by linking to the
presubmit script so it will automatically run before pushing a branch to the remote
GitHub repository.

To add the presubmit script as a prepush hook:

```
# From Repository Root
ln -s -f ../../scripts/presubmit.sh .git/hooks/pre-push
chmod a+x .git/hooks/pre-push
```

## Source and build

### Source code layout

Common code is in the `/pkg` folder.

Each binary will have its `main.go` file in a `/cmd/[bin-name]` folder.

## Project dependencies

1. Protocol Buffer compiler.

    To install the Protocol Buffer compiler:

    [Windows and Linux binaries, and source code](https://github.com/protocolbuffers/protobuf/releases)

    OS Managed binaries:

    | OS       | Command                                            |
    |----------|----------------------------------------------------|
    | Mac OS X ([Brew](https://brew.sh/)) | `brew install protobuf` |
    | Linux (APT) | `apt-get install protobuf-compiler`             |
    | Linux (YUM) | `yum install protobuf-compiler`                 |

1. The protoc-gen-go module.

    To install protoc-gen-go:

    1. Clone the Go Protocol Buffer module repository

        ```
        git clone https://github.com/golang/protobuf
        ```

    1. Build the module:

        ```
        cd protobuf/protoc-gen-go
        go build
        ```

    1. Move the binary to a folder defined in your `PATH` environment variable, for example `$HOME/bin`

        ```
        mv protoc-gen-go $HOME/bin
        ```

### Running locally

1. (One time only) Create a dev service account and add the credentials to `./local/sa.json`

2. Setup env

```
source scripts/setup_env.sh
```

3. Run with go

```
go run ./cmd/[bin-name]
```

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