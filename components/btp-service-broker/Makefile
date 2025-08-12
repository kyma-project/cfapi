ENVTEST_K8S_VERSION = 1.24.1
REGISTRY = ghcr.io
IMG = kyma-project/cfapi/btp-service-broker
VERSION = 0.0.0
RELEASE_DIR =release/$(VERSION)


.PHONY: test
test: envtest fmt vet
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go run github.com/onsi/ginkgo/v2/ginkgo -r --output-interceptor-mode=none --randomize-all --randomize-suites

.PHONY: docker-build
docker-build:
	docker build --build-arg VERSION=${VERSION} -t ${REGISTRY}/${IMG} .
	docker tag ${REGISTRY}/${IMG} ${REGISTRY}/${IMG}:${VERSION}

.PHONY: docker-push
docker-push:
ifneq (,$(GCR_DOCKER_PASSWORD))
	docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
endif
	docker push --all-tags ${REGISTRY}/${IMG}

.PHONY: release
release: bin/yq
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)
	cp -a helm $(RELEASE_DIR)
	IMG_SHA=$(shell docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${IMG}) yq -i 'with(.broker; .image=env(IMG_SHA))' $(RELEASE_DIR)/helm/values.yaml

##@ Tools
########## static code checks ###########
.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

lint: golangci-lint

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

########## envtest ###########
ENVTEST ?= $(LOCALBIN)/setup-envtest
.PHONY: envtest
envtest: $(ENVTEST) ## Download & Build envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

bin:
	mkdir -p bin

bin/vendir:
	go install carvel.dev/vendir/cmd/vendir@latest

vendir: bin/vendir
	vendir sync --chdir tests

bin/yq: bin
	go install github.com/mikefarah/yq/v4@latest

bin/golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

golangci-lint: bin/golangci-lint
	golangci-lint run
