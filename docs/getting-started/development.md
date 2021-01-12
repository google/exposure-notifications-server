---
layout: default
---

# Local development

This guide covers how to develop the system locally. Note that the configuration
options described in this page are optimized for local development and may not
represent best practices.

1.  Install [gcloud](https://cloud.google.com/sdk).

1.  Create a Google Cloud project using the Cloud Console. Set your **Project
    ID** as an environment variable:

    ```sh
    export PROJECT_ID="..."
    ```

    Note this is the project _ID_, not _name_ or _number_.

1.  Configure gcloud:

    ```sh
    gcloud auth login && \
    gcloud auth application-default login && \
    gcloud auth application-default set-quota-project "${PROJECT_ID}"
    ```

1.  Change directory into this repository:

    ```text
    cd /path/to/exposure-notifications-server
    ```

1.  Create a `.env` file with your configuration. This will aid future
    development since you can `source` this file instead of trying to find all
    these values again.

    ```sh
    # Google project configuration.
    export PROJECT_ID="TODO"
    export GOOGLE_CLOUD_PROJECT="${PROJECT_ID}"

    # Disable observability locally.
    export OBSERVABILITY_EXPORTER="NOOP"

    # Configure local logging.
    export LOG_LEVEL="debug"
    export LOG_MODE="development"

    # Configure key management for revision tokens. Create your own revision
    # token AAD with:
    #
    #     openssl rand -base64 16
    #
    export KEY_MANAGER="FILESYSTEM"
    export KEY_FILESYSTEM_ROOT="$(pwd)/local/keys"
    export REVISION_TOKEN_AAD="48W/fnGCagSiEW8j8hanTQ=="
    export REVISION_TOKEN_KEY_ID="system/revision-token-encrypter"

    # Configure secrets management.
    export SECRET_MANAGER="IN_MEMORY"
    export SECRETS_DIR="$(pwd)/local/secrets"

    # Configure blobstore.
    export BLOBSTORE="MEMORY"

    # Don't cache authorized apps in development.
    export AUTHORIZED_APP_CACHE_DURATION="1s"

    # Development config for export
    export EXPORT_FILE_MIN_RECORDS="1"
    export TRUNCATE_WINDOW="1m"
    export MIN_WINDOW_AGE="1m"

    # Development config for exposure (publish)
    export ALLOW_PARTIAL_REVISIONS="true"
    export DEBUG_RELEASE_SAME_DAY_KEYS="true"
    export DEBUG_LOG_BAD_CERTIFICATES="true"

    # Development config for key rotation
    export NEW_KEY_PERIOD="1s"
    export DELETE_OLD_KEY_PERIOD="30s"

    # Configure database pooling
    export DB_POOL_MIN_CONNS="2"
    export DB_POOL_MAX_CONNS="10"

    # Enable dev mode
    export DEV_MODE="true"
    export DB_DEBUG="true"
    ```

1.  Source the `.env` file. Do this each time before you start the server:

    ```sh
    source .env
    ```

1.  Start the database:

    ```sh
    eval $(./scripts/dev init)
    ./scripts/dev dbstart
    ```

1.  Run any migrations:

    ```sh
    ./scripts/dev dbmigrate
    ```

1.  (Optional) Seed the database with fake data:

    ```sh
    ./scripts/dev dbseed
    ```

    This will create some default data in the system including two apps and a
    testing health authority.

1.  Start the server:

    ```sh
    go run ./cmd/exposure
    ```
