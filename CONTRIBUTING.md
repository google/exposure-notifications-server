# Exposure Server Components

Contributions to this project are welcomed and encouraged. We request that you
read through the guidelines before diving in.

## Contributor License Agreement

Contributions to this project must be accompanied by a Contributor License
Agreement (CLA). You, or your employer, retain the copyright to your
contribution. This agreement simply gives us permission to use and redistribute your
contributions as part of the project. You can see existing accepted CLAs or sign
a new agreement at the
[Contributor License Agreement webpage](https://cla.developers.google.com/).

You generally only need to submit one CLA, so if you've already submitted one
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
is located at scripts/presubmit.sh. You can add a prepush hook by linking to the
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

Common code is in the `/pkg` folder

Each binary will have main in `/cmd/[bin-name]`

## Project dependencies

1. Protocol Buffers.

    To install the Protocol Buffer library:

      * Mac OS X:
        
        Using [Brew](https://brew.sh/):

           `brew install protobuf`
      
      * Linux:
        
        Using the APT package manager:

           `apt-get install protobuf-compiler` 

        Using the YUM package manager:

           `yum install protobuf-compiler`

       * Source: https://github.com/protocolbuffers/protobuf/releases

1. The protoc-gen-go library.

   To install protoc-gen-go:

     `go get -u github.com/golang/protobuf/protoc-gen-go`

1. The goimports tool.
  
   To install goimports:
   
     `go install golang.org/x/tools/cmd/goimports`

1. OpenCensus 

   To install the OpenCencus library:

       `go get -u go.opencensus.io`

1. You may need to update your path to include these libraries and tools:

```
export PATH=$PATH:$HOME/go/bin, in order to add the GOPATH and
export PATH=$PATH:/usr/local/go/bin, in order to add GOROOT
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
