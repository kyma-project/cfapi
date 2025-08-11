ENVTEST_K8S_VERSION = 1.24.1
REGISTRY = ghcr.io
IMG ?= kyma-project/cfapi/cfapi-broker
VERSION ?= 0.0.0


.PHONY: test
test: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" ginkgo -r -coverprofile cover.out

.PHONY: docker-build
docker-build:
	docker build -t ${REGISTRY}/${IMG} .
	docker tag ${REGISTRY}/${IMG} ${REGISTRY}/${IMG}:${VERSION}

.PHONY: docker-push
docker-push:
ifneq (,$(GCR_DOCKER_PASSWORD))
	docker login $(IMG_REGISTRY) -u oauth2accesstoken --password $(GCR_DOCKER_PASSWORD)
endif
	docker push --all-tags ${REGISTRY}/${IMG}

.PHONY: release
release: bin/yq
	rm -rf release
	mkdir -p release
	$(eval IMG_SHA = $(shell docker inspect --format='{{index .RepoDigests 0}}' ${REGISTRY}/${IMG}))
	cp -a helm release
	IMG_SHA=$(IMG_SHA) yq -i 'with(.broker; .image=env(IMG_SHA))' release/helm/values.yaml

##@ Tools

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
