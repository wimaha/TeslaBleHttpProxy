.DEFAULT_GOAL := default

IMAGE ?= wimaha/tesla-ble-http-proxy
VERSION := 1.2.7

export DOCKER_CLI_EXPERIMENTAL=enabled

.PHONY: build # Build the container image
build:
	@docker buildx create --use --name=crossplat --node=crossplat && \
	docker buildx build \
		--output "type=docker,push=false" \
		--tag $(IMAGE) \
		.

.PHONY: publish # Push the image to the remote registry
publish:
	@docker buildx create --use --name=crossplat --node=crossplat && \
	docker buildx build \
		--platform linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le \
		--output "type=image,push=true" \
		--tag $(IMAGE) \
		--tag $(IMAGE):$(VERSION) \
		--tag $(IMAGE):dev \
		.

.PHONY: dev # Push the image to the remote registry
dev:
	@docker buildx create --use --name=crossplat --node=crossplat && \
	docker buildx build \
		--platform linux/386,linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64,linux/ppc64le \
		--output "type=image,push=true" \
		--tag $(IMAGE):dev \
		.