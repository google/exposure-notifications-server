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

Please note that the `federation*` packages are reference-only, and we do not 
actively support them.

## Source and build

### Source code layout

Common code is in the `/internal` folder.

Each binary will have its `main.go` file in a `/cmd/[bin-name]` folder.

### Installing project dependencies

To run the server, you must install the following dependencies:

1.  [Go 1.16 or newer](https://golang.org/dl/).

1.  [Docker][docker].

### Running tests

Run the tests with:

```text
$ go test ./...
```

### Presubmit checks

You should run the presubmit checks before committing changes. The presubmit script
is located at `scripts/presubmit.sh`.

### Running locally

These instructions use [Docker][docker] to run components locally. You may be
able to run these components without Docker, but these instructions assume
Docker is installed and available in your `$PATH`.

1.  Set development environment variables:

    ```text
    $ eval $(./scripts/dev init)
    ```

    **If you close your terminal tab or session, you need to re-run this
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


[docker]: https://docs.docker.com/get-docker/
