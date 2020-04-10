# CT Server Components

# Code location.

Check out the code to `$GOPATH/src/cambio` on your machine.

# Layout

Common code goes in `/pkg`

Each binary will have main in `/cmd/[bin-name]`

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
