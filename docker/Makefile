# Copyright 2020 the Exposure Notifications Server authors
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

PROJECT := cloud-devrel-public-resources
NAMESPACE := exposure-notifications
TAG := $(shell date +v%Y%m%d)-$(shell git describe --tags --always --dirty)

.PHONY: build-%
build-%:
	@docker build \
		--tag gcr.io/$(PROJECT)/$(NAMESPACE)/$(*):$(TAG) \
		--tag gcr.io/$(PROJECT)/$(NAMESPACE)/$(*):latest \
		--file $(*).dockerfile \
		../
	@docker push gcr.io/$(PROJECT)/$(NAMESPACE)/$(*):$(TAG)
	@docker push gcr.io/$(PROJECT)/$(NAMESPACE)/$(*):latest
