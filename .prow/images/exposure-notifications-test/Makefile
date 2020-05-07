PROJECT := apollo-server-273118
IMAGE := exposure-notifications-test
TAG := $(shell date +v%Y%m%d)-$(shell git describe --tags --always --dirty)

.PHONY: create-image
create-image:
	gcloud auth configure-docker
	docker build -t $(IMAGE):$(TAG) -t $(IMAGE):latest -f Dockerfile .

.PHONY: update-image
update-image:
	gcloud auth configure-docker
	docker tag $(IMAGE):$(TAG) gcr.io/$(PROJECT)/$(IMAGE):$(TAG)
	docker tag $(IMAGE):latest gcr.io/$(PROJECT)/$(IMAGE):latest
	docker push gcr.io/$(PROJECT)/$(IMAGE):$(TAG)
	docker push gcr.io/$(PROJECT)/$(IMAGE):latest

.PHONY: push-image
push-image: create-image update-image
