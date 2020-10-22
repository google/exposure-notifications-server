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

# Stable as of 10/22/2020
FROM chromedp/headless-shell:86.0.4240.111 as headlessshell

FROM golang:1.15.2

# Install sudo
RUN apt-get update -yqq && apt-get install -yqq sudo unzip

# Install terraform
RUN wget -q https://releases.hashicorp.com/terraform/0.13.1/terraform_0.13.1_linux_amd64.zip \
  && unzip terraform_0.13.1_linux_amd64.zip \
  && mv terraform /usr/bin \
  && rm terraform_0.13.1_linux_amd64.zip

# Install jq
RUN curl -o /usr/bin/jq http://stedolan.github.io/jq/download/linux64/jq \
  && chmod +x /usr/bin/jq

# Install gcloud
WORKDIR /workspace
RUN mkdir -p /workspace

ENV PATH=/google-cloud-sdk/bin:/workspace:${PATH} \
    CLOUDSDK_CORE_DISABLE_PROMPTS=1

RUN wget -q https://dl.google.com/dl/cloudsdk/channels/rapid/google-cloud-sdk.tar.gz && \
    tar xzf google-cloud-sdk.tar.gz -C / && \
    rm google-cloud-sdk.tar.gz && \
    /google-cloud-sdk/install.sh \
        --disable-installation-options \
        --bash-completion=false \
        --path-update=false \
        --usage-reporting=false && \
    gcloud components install alpha beta kubectl && \
    gcloud info | tee /workspace/gcloud-info.txt

#
# BEGIN: DOCKER IN DOCKER SETUP
#

# Install Docker deps
RUN apt-get update -yqq && apt-get install -yqq --no-install-recommends \
    apt-transport-https \
    ca-certificates \
    curl \
    gnupg2 \
    software-properties-common \
    lsb-release && \
    rm -rf /var/lib/apt/lists/*

# Add the Docker apt-repository
RUN curl -fsSL https://download.docker.com/linux/$(. /etc/os-release; echo "$ID")/gpg \
    | apt-key add - && \
    add-apt-repository \
    "deb [arch=amd64] https://download.docker.com/linux/$(. /etc/os-release; echo "$ID") \
    $(lsb_release -cs) stable"

# Install Docker
RUN apt-get update -yqq && \
    apt-get install -yqq --no-install-recommends docker-ce=5:19.03.* && \
    rm -rf /var/lib/apt/lists/* && \
    sed -i 's/cgroupfs_mount$/#cgroupfs_mount\n/' /etc/init.d/docker \
    && update-alternatives --set iptables /usr/sbin/iptables-legacy \
    && update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy

# Move Docker's storage location
RUN echo 'DOCKER_OPTS="${DOCKER_OPTS} --data-root=/docker-graph"' | \
    tee --append /etc/default/docker

# NOTE this should be mounted and persisted as a volume
RUN mkdir /docker-graph

# Install entrypoint to support
RUN curl -sfLo "/bin/runner.sh" "https://raw.githubusercontent.com/kubernetes/test-infra/master/images/bootstrap/runner.sh" && \
  chmod +x "/bin/runner.sh"

#
# END: DOCKER IN DOCKER SETUP
#

# Install goimports
RUN go get -u github.com/client9/misspell/cmd/misspell
RUN go get -u golang.org/x/tools/cmd/goimports
RUN go get -u honnef.co/go/tools/cmd/staticcheck
# GCP projects pool manager
RUN go get -u sigs.k8s.io/boskos/cmd/boskosctl

# Copy headless-shell and add it to PATH
WORKDIR /headless-shell
RUN mkdir -p /headless-shell
COPY --from=headlessshell /headless-shell/headless-shell /headless-shell/headless-shell
# "libnss3.so" under /usr/lib is required by headless-shell, also copy this
COPY --from=headlessshell /usr/lib/* /usr/lib/
ENV PATH=/headless-shell:/workspace:${PATH}

ENTRYPOINT ["/bin/runner.sh"]
