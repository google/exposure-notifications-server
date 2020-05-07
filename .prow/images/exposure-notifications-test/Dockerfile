FROM golang:1.14-buster

RUN apt-get update
RUN apt-get install -y --no-install-recommends \
    unzip

# Install protoc components
RUN apt-get install -y protobuf-compiler
RUN git clone https://github.com/golang/protobuf; \
	cd protobuf/protoc-gen-go; \
	go build; \
	mv protoc-gen-go $HOME/bin
