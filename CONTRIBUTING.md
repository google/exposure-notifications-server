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

### Project dependencies

1. Go v1.14.0 or higher.

    Go can be downloaded from https://golang.org/dl/

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

### Running tests

You can run the same tests that are used in the continuous integration pipeline
by running:

```
./scripts/kokoro_presubmit.sh "."
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

1. (One time only) Create a dev service account and add the credentials to `./local/sa.json`

1. Setup env

    ```
    source scripts/setup_env.sh
    ```

1. Create a postgres db locally.

    ```
    psql postgres
    postgres-# CREATE ROLE apollo WITH LOGIN PASSWORD 'mypassword';
    postgres-# CREATE DATABASE apollo;
    postgres=# GRANT ALL PRIVILEGES ON DATABASE apollo TO apollo;
    postgres=# \q
    ```

1. Configure Database Schema and Run Migrations

    ```
    psql $DB_USER -h $DB_HOST -d $DB_DBNAME -f scripts/schema.sql
    ./scripts/run_db_migrations.sh up
    ```

1. Run with go

    ```
    go run ./cmd/[bin-name]
    ```

## Documentation

User documentation for this project is in the [`docs`](/docs/index.md) directory,
with information on building, deploying and using the reference implementation.