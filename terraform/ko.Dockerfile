FROM golang
RUN go get github.com/google/ko/cmd/ko
ENTRYPOINT ["ko"]
