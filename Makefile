# Image URL to use all building/pushing image targets
VERSION = 0.0.0
REGISTRY = ghcr.io
IMG = kyma-project/cfapi/cfapi-controller

RELEASE_DIR ?= release/$(VERSION)
CFAPI_RELEASE_DIR ?= $(RELEASE_DIR)/cfapi
BTP_SERVICE_BROKER_RELEASE_DIR ?= $(RELEASE_DIR)/btp-service-broker

export GOBIN = $(shell pwd)/bin
export PATH := $(shell pwd)/bin:$(PATH)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

manifests: bin/controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	controller-gen crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

generate: bin/controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."

test: manifests generate fmt vet
	make -C components/btp-service-broker fmt vet test

docker-build: ## Build docker image with the manager.
	docker build -t ${REGISTRY}/${IMG} --build-arg TARGETARCH=amd64 --build-arg BTP_SERVICE_BROKER_RELEASE_DIR=$(BTP_SERVICE_BROKER_RELEASE_DIR) .
	docker tag ${REGISTRY}/${IMG} ${REGISTRY}/${IMG}:${VERSION}

docker-push: ## Push docker image with the manager.
ifneq (,$(GCR_DOCKER_PASSWORD))
	docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
endif
	docker push ${REGISTRY}/${IMG}:${VERSION}

##@ Release

release: bin/kustomize manifests btp-service-broker-release cfapi-release

btp-service-broker-release:
	rm -rf $(BTP_SERVICE_BROKER_RELEASE_DIR)
	mkdir -p $(BTP_SERVICE_BROKER_RELEASE_DIR)
	make -C components/btp-service-broker docker-build REGISTRY=$(REGISTRY) VERSION=$(VERSION)
	make -C components/btp-service-broker docker-push REGISTRY=$(REGISTRY) VERSION=$(VERSION)
	make -C components/btp-service-broker release REGISTRY=$(REGISTRY) RELEASE_DIR=$(shell pwd)/$(BTP_SERVICE_BROKER_RELEASE_DIR)

cfapi-release: bin/kustomize docker-build docker-push
	rm -rf $(CFAPI_RELEASE_DIR)
	mkdir -p $(CFAPI_RELEASE_DIR)

	$(shell mkdir -p $(RELEASE_DIR)/tmp && cp -a config $(RELEASE_DIR)/tmp)

	cp default-cr.yaml $(CFAPI_RELEASE_DIR)/cfapi-default-cr.yaml

	$(eval IMG_SHA = $(shell docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${IMG}))
	pushd $(RELEASE_DIR)/tmp/config/manager && kustomize edit set image controller=$(IMG_SHA) && popd
	pushd $(RELEASE_DIR)/tmp/config/manager && kustomize edit add label app.kubernetes.io/version:$(VERSION) --force --without-selector --include-templates && popd
	kustomize build $(RELEASE_DIR)/tmp/config/default > $(CFAPI_RELEASE_DIR)/cfapi-operator.yaml

fmt: ## Run go fmt against code.
	go fmt ./...

vet: ## Run go vet against code.
	go vet ./...

lint: fmt vet golangci-lint

bin:
	mkdir -p bin

clean-bin:
	# envtest binaries lack the write permissions, chmod them before deleting
	find . -name "testbin" -type d -exec chmod -R +w '{}' \;
	# globstar (e.g. in rm -f **/bin/*) isn't available in the version of bash packaged with MacOS
	find . -wholename '*/testbin/*' -delete
	find . -wholename '*/bin/*' -delete

bin/golangci-lint: bin
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

golangci-lint: bin/golangci-lint
	make -C components/btp-service-broker lint
	golangci-lint run

bin/kustomize: bin
	go install sigs.k8s.io/kustomize/kustomize/v5@latest

bin/controller-gen: bin
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0
