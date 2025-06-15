# build vars
TAG_NAME := $(shell test -d .git && git describe --abbrev=0 --tags)
SHA := $(shell test -d .git && git rev-parse --short HEAD)
VERSION := $(if $(TAG_NAME),$(TAG_NAME),$(SHA))

LD_FLAGS := -X github.com/wimaha/TeslaBleHttpProxy/config.Version=$(VERSION) -s -w
BUILD_ARGS := -ldflags='$(LD_FLAGS)'
BUILD_DATE := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# docker
DOCKER_IMAGE := wimaha/tesla-ble-http-proxy
DOCKER_TAG := dev
PLATFORM := linux/amd64,linux/arm64,linux/arm/v6,linux/arm/v7

export DOCKER_CLI_EXPERIMENTAL=enabled

default:: build

lint::
	golangci-lint run

build::
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	go build $(BUILD_ARGS)

build-docker::
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	go build $(BUILD_ARGS) -o /go/bin/teslaBleHttpProxy main.go

docker::
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	docker buildx build --tag $(DOCKER_IMAGE) --output "type=docker,push=false" . 
#--progress=plain --no-cache 

dev::
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	docker buildx build --platform $(PLATFORM) --tag $(DOCKER_IMAGE):$(DOCKER_TAG) --output "type=image,push=true" .

publish::
	@echo Version: $(VERSION) $(SHA) $(BUILD_DATE)
	docker buildx build --platform $(PLATFORM) --tag $(DOCKER_IMAGE) --tag $(DOCKER_IMAGE):$(VERSION) --tag $(DOCKER_IMAGE):$(DOCKER_TAG) --output "type=image,push=true" .
