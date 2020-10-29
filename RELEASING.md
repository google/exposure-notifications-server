# Exposure Notifications Servers Release Process

  * Releases are cut every other Tuesday (target time 9am, US Pacific Time)
  * All releases will have a release branch and a corresponding tag
  * Both the exposure-notifications-sever and exposure-notifications-verification-server will follow the same release numbering

Most issues will not be backported to previous releases. It is recommended that server
operators keep up with the release cadence of the open source project as much as possible.

# Dependencies

You need the kubernetes release-notes tool to generate release notes.

```shell
GO111MODULE=on go get k8s.io/release/cmd/release-notes
```

You'll also need a GitHub Personal Access Token with permission to read repositories. It should be exported as `GITHUB_TOKEN`.

You should have a [gpg signing key](https://docs.github.com/en/github/authenticating-to-github/telling-git-about-your-signing-key) set up for your github account.

# Cutting a release

Set a target version:

```shell
# Note: no "v" prefix here!
export RELEASE_VERSION="0.3.0"
```

## Generate release notes

This is a diff from the last time a release was generated.

```shell
export LAST_RELEASE_TAG="TODO" # e.g. v0.2.2

release-notes \
  --org "google" \
  --repo "exposure-notifications-server" \
  --branch "main" \
  --required-author ""  \
  --start-rev "${LAST_RELEASE_TAG}" \
  --end-rev "main" \
  --output "/tmp/relnotes-${RELEASE_VERSION}.md" \
  --repo-path "/tmp/relnotes-repo" \
  --dependencies true
```

After you have complete release notes, you may want to switch your repo-path or clean up.

```shell
rm -rf /tmp/relnotes-repo
```

## Update the release notes

Edit the generated relnotes file and add anything that may have been missed. It's also wise to call out
database migrations, possible breaking changes, and new environment variables that are are needed.

## Create a release branch

Ensure that you are on `main`, that you are up to date (the commit you want to cut the release at). This requires repo admin privileges.

```shell
git checkout -b release-${RELEASE_VERSION%??}
git push --set-upstream origin release-${RELEASE_VERSION%??}
```

If you are cutting a patch release, the `release-x.y` branch likely already exists:

```shell
git fetch --all
git checkout -t origin/release-${RELEASE_VERSION%??}
# cherry-pick or decide the best way to bring over changes
git push origin release-${RELEASE_VERSION%??}
```

## Create and push tag

Ensure you are on the **release branch** and create a new tag:

```shell
git tag -a -s -m "Release v${RELEASE_VERSION}" v${RELEASE_VERSION}
git push origin --tags
```

## Draft a new release on GitHub

1. Find the tag you just pushed on the repo:

    - [main](https://github.com/google/exposure-notifications-server/tags)
    - [verification](https://github.com/google/exposure-notifications-verification-server/tags)

1. Click the "..." and choose "create release".

1. Copy the release notes into the web form.

# Go do the same on exposure-notifications-verification-server

Before releasing, update the `go.mod` file for the exposure-notifications-verification-server
so that it references the exposure-notifications-server version that was just released:

```text
# From github.com/google/exposure-notifications-verification-server
go get -u github.com/google/exposure-notifications-server@v${RELEASE_VERSION}

# Cleanup
go mod tidy
go mod verify
```

The version being tracked may have been un-pined since the last release or just should
generally be updated to keep the projects in sync.
