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

FROM alpine
RUN apk add --no-cache bash

ADD https://storage.googleapis.com/cloudsql-proxy/v1.19.1/cloud_sql_proxy.linux.amd64 /cloud-sql-proxy
COPY ./bin/migrate /migrate
COPY ./builders/cloud-sql-exec /cloud-sql-exec
COPY ./migrations /migrations

RUN chown $(whoami):$(whoami) /cloud-sql-proxy /cloud-sql-exec /migrate
RUN chmod +x /cloud-sql-proxy /cloud-sql-exec /migrate

ENTRYPOINT ["/cloud-sql-exec"]
CMD ["/migrate"]
