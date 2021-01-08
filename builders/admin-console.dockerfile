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

FROM alpine AS builder
RUN mkdir -p /var/run/secrets && \
  chmod 0700 /var/run/secrets && \
  chown 65534:65534 /var/run/secrets

FROM scratch
ARG SERVICE
COPY ./builders/passwd /etc/passwd
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

USER nobody
COPY ./bin/${SERVICE} /server
COPY --from=builder /var/run /var/run
COPY --from=builder /var/run/secrets /var/run/secrets

ENV PORT 8080
ENTRYPOINT ["/server"]
