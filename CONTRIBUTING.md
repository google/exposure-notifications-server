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

## Source and build

### Source code layout

Common code is in the `/pkg` folder.

Each binary will have its `main.go` file in a `/cmd/[bin-name]` folder.

### Installing project dependencies

To run the server, you must install the following dependencies:

1. [Go 1.14.0 or newer](https://golang.org/dl/).

1. The Protocol Buffer compiler.

    To install the Protocol Buffer compiler:

    [Windows and Linux binaries, and source code](https://github.com/protocolbuffers/protobuf/releases)

    OS-managed binaries:

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

    1. Move the binary to a folder defined in your `PATH` environment variable, such as `$HOME/bin`

        ```
        mv protoc-gen-go $HOME/bin
        ```

### Running tests

You can run the same tests that are used in the continuous integration pipeline
by running:

```
./scripts/presubmit.sh "."
```

You can also use `go test`:

```
go test ./...
```

To run database tests, install Postgres or start a server with Docker (see
"Running locally" later in this topic), and then set some environment variables:

```
DB_SSLMODE=disable DB_USER=postgres go test -v ./internal/database
```


### Presubmit checks

You should run the presubmit checks before committing changes. The presubmit script
is located at `scripts/presubmit.sh`.

You can add a prepush hook by linking to the presubmit script to automatically
run before pushing a branch to the remote GitHub repository. To add the
presubmit script as a prepush hook, go to the root directory of the repository
and type:

```
ln -s -f ../../scripts/presubmit.sh .git/hooks/pre-push
chmod a+x .git/hooks/pre-push
```

### Running locally

These instructions use [Docker](https://docs.docker.com/get-docker/) to run
components locally. You may be able to run these components without Docker, but
these instructions assume Docker is installed and available in your `$PATH`.

1.  Set development environment variables:

    ```text
    $ eval $(./scripts/dev init)
    ```

    **If you close your terminal tab or session, you will need to re-run this
    command.**

1.  Create the local development database:

    ```text
    $ ./scripts/dev dbstart
    ```

    This command may take a few minutes to execute on the first invocation. This
    is because it needs to download the Postgres container. Future invocations
    will be faster.

1.  Run any migrations. This creates the tables and schema:

    ```text
    $ ./scripts/dev dbmigrate
    ```

1.  (Optional) Seed the database with some initial data:

    ```text
    $ ./scripts/dev dbseed
    ```

1.  Run a component. For example, to run the `exposure` endpoint:

    ```text
    $ go run ./cmd/exposure/...
    ```

1.  When you're done developing, you can stop the database.

    ```text
    $ ./scripts/dev dbstop
    ```

    **Warning: This will also delete any stored data in the database.**


## Documentation

User documentation for this project is in the [`docs`](/docs/index.md) directory,
with information on building, deploying, and using the reference implementation.
