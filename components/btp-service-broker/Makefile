REGISTRY = ghcr.io
IMG = kyma-project/cfapi/btp-service-broker
VERSION = 0.0.0
RELEASE_DIR =release/$(VERSION)

export GOBIN = $(shell pwd)/bin
export PATH := $(shell pwd)/bin:$(PATH)

test: fmt vet bin/setup-envtest
	KUBEBUILDER_ASSETS="$(shell setup-envtest use -p path)" go run github.com/onsi/ginkgo/v2/ginkgo -r --output-interceptor-mode=none --randomize-all --randomize-suites

docker-build:
	docker build --build-arg VERSION=${VERSION} -t ${REGISTRY}/${IMG} .
	docker tag ${REGISTRY}/${IMG} ${REGISTRY}/${IMG}:${VERSION}

docker-push:
ifneq (,$(GCR_DOCKER_PASSWORD))
	docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
endif
	docker push --all-tags ${REGISTRY}/${IMG}

release: bin/yq
	rm -rf $(RELEASE_DIR)
	mkdir -p $(RELEASE_DIR)
	cp -a helm $(RELEASE_DIR)
	IMG_SHA=$(shell docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${IMG}) yq -i 'with(.broker; .image=env(IMG_SHA))' $(RELEASE_DIR)/helm/values.yaml

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet golangci-lint

bin:
	mkdir -p bin

bin/yq: bin
	go install github.com/mikefarah/yq/v4@latest

bin/golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

golangci-lint: bin/golangci-lint
	golangci-lint run

bin/setup-envtest: bin/testbin
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

bin/testbin: bin/setup-envtest
	mkdir -p bin/testbin
