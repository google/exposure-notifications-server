# Exposure Server Components

# How to Contribute

We'd love to accept your patches and contributions to this project. There are
just a few small guidelines you need to follow.

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

## Code reviews

All submissions, including submissions by project members, require review. We
use GitHub pull requests for this purpose. Consult
[GitHub Help](https://help.github.com/articles/about-pull-requests/) for more
information on using pull requests.

It is strongly recommended that you run the presubmits before publishing. You
can find a script to do this at scripts/presubmit.sh

You can also add a prepush hook by linking to the script. This will run
these automatically before pushing a branch to the GitHub remote.

```
# From Repository Root
❯ ln -s -f ../../scripts/presubmit.sh .git/hooks/pre-push
❯ chmod a+x .git/hooks/pre-push
```

## Community Guidelines

This project follows
[Google's Open Source Community Guidelines](https://opensource.google/conduct/).

# Additional Project Details
## Code Layout

Common code goes in `/pkg`

Each binary will have main in `/cmd/[bin-name]`

## Project Dependencies
1. This project requires protoc.
  - Mac: `brew install protobuf`
  - Linux: `apt-get install protobuf-compiler`
  - Source: https://github.com/protocolbuffers/protobuf/releases
1. Install protoc-gen-go `go get -u github.com/golang/protobuf/protoc-gen-go`
1. Install `go install golang.org/x/tools/cmd/goimports`
1. Install OpenCensus `go get -u go.opencensus.io`
1. You may need to update your path to include these tools

```
export PATH=$PATH:$HOME/go/bin, in order to add the GOPATH and
export PATH=$PATH:/usr/local/go/bin, in order to add GOROOT
```

## Running locally

1. (One time only) Create a dev service account and add the credentials to `./local/sa.json`

2. Setup env

```
source scripts/setup_env.sh
```

3. Run with go

```
go run ./cmd/[bin-name]
```



## Building / publishing images

1. Install ko

```
GO111MODULE=on go get github.com/google/ko/cmd/ko
```

2. Configure ko

```
source setup_ko.sh
```

3. Generate GCR docker config

```
gcloud auth configure-docker
```

4. Build and publish the desired container

For example, to publish the infection server.

```
ko publish ./cmd/infection
```
