# CT Server Components

# Code location.

Check out the code to `$GOPATH/src/cambio` on your machine.

# Layout

Common code goes in `/pkg`

Each binary will have main in `/cmd/[bin-name]`

# Dependencies
1. This project requires protoc.
  - Mac: `brew install protobuf`
  - Linux: `apt-get install protobuf-compiler`
  - Source: https://github.com/protocolbuffers/protobuf/releases
1. Install protoc-gen-go `go get -u github.com/golang/protobuf/protoc-gen-go`
1. Install `go install golang.org/x/tools/cmd/goimports`
1. You may need to update your path to include these tools

```
export PATH=$PATH:$HOME/go/bin, in order to add the GOPATH and
export PATH=$PATH:/usr/local/go/bin, in order to add GOROOT
```

# Running locally

1. (One time only) Create a dev service account and add the credentials to `./local/sa.json`

2. Setup env

```
source scripts/setup_env.sh
```

3. Run with go

```
go run ./cmd/[bin-name]
```

# Code Reviews

1. Before creating a code review, you can run the presubmits at scripts/presubmit.sh

1. You can add a prepush hook by linking to the script:

```
# From Repository Root
❯ ln -s -f ../../scripts/presubmit.sh .git/hooks/pre-push
❯ chmod a+x .git/hooks/pre-push
```


# Building / publishing images

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
