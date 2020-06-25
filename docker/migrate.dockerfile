# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# migrate is a base container with migrate and the Cloud SQL proxy installed.

FROM golang:1.14 AS builder

# Install deps
RUN apt-get -qq update && apt-get -yqq install upx

# Install proxy
RUN wget -q https://dl.google.com/cloudsql/cloud_sql_proxy.linux.amd64 -O /bin/cloud_sql_proxy
RUN chmod +x /bin/cloud_sql_proxy

# Install go-migrate
RUN go get -tags 'postgres' -u github.com/golang-migrate/migrate/cmd/migrate

ENV GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64

WORKDIR /src
COPY . .

RUN go build \
  -trimpath \
  -ldflags "-s -w -extldflags '-static'" \
  -installsuffix cgo \
  -tags netgo \
  -o /bin/migrate \
  ./cmd/migrate

RUN strip /bin/migrate
RUN upx -q -9 /bin/migrate

FROM alpine
RUN apk add --no-cache bash ca-certificates

COPY --from=builder /bin/cloud_sql_proxy /bin/cloud_sql_proxy
COPY --from=builder /go/bin/migrate /bin/gomigrate
COPY --from=builder /bin/migrate /bin/migrate

ENTRYPOINT ["/bin/migrate"]
