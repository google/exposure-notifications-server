# Exposure Notifications Servers Release Process

  * Releases are cut every Tuesday (target time 9am, US Pacific Time)
  * All releases will have a release branch and a corresponding tag
  * Both the exposure-notifications-sever and exposure-notifications-verification-server wil follow the same release numbering

# Dependenceis

You need the kubernetes release-notes tool to generate release notes.

```shell
GO111MODULE=on go get k8s.io/release/cmd/release-notes
```

# Cutting a release

## Generate release notes

This is a diff from the last time a release was generated. Adjust the `start-rev` and filename output as appropriate. The start rev should be the last release tag.

```
release-notes --github-org google --github-repo exposure-notifications-server --required-author="" --start-rev=v0.2.0 --end-rev=main --output=/tmp/relnotes-0.3.0.md --repo-path=/tmp/relnotes-repo --dependencies=true  --branch=main
```

## Update the release notes

Edit the generated relnotes file and add anything that may have been missed. It's also wise to call out
database migrations, possible breaking changes, and new environment variables that are are needed.

## Create a release branch

Ensure that you are on main, that you are up to date (the commit you want to cut the release at). This requires repo admin privelages.

```
git branch release-0.3
git checkout release-0.3
git push --set-upstream upstream release-0.3
```

## Draft a new release on github

Visit: https://github.com/google/exposure-notifications-server/releases/new

Set the tag, including the patch level, i.e. `v0.3.0`

Copy the release notes into the web form.

# Go do the same on exposure-notifications-verification-server