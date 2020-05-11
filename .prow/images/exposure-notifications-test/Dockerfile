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

# This image is used to run ./scripts/presubmit.sh on CI

FROM golang:1.14-buster

RUN apt-get update;

# Install Postgres
RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ buster-pgdg main" >> /etc/apt/sources.list.d/pgdg.list; \
	wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc |  apt-key add -; \
	apt-get update; \
	apt-get install -y postgresql-12 postgresql-client-12

# Configure Postgres, password for test db user, set a timezone
RUN ln -sf /usr/share/zoneinfo/US/Pacific /etc/localtime
RUN service postgresql start; \
	runuser -l postgres -c "psql -c \"ALTER USER postgres WITH PASSWORD 'mypassword'\" -d postgres;"

# Install protoc components
RUN apt-get install -y protobuf-compiler
RUN git clone https://github.com/golang/protobuf; \
	cd protobuf/protoc-gen-go; \
	go build; \
	mv protoc-gen-go $GOPATH/bin; \
	export PATH=$PATH:$GOPATH/bin/

RUN go get golang.org/x/tools/cmd/goimports
