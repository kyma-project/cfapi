ENVTEST_K8S_VERSION = 1.24.1
IMAGE ?= kyma-project/cfapi/cfapi-broker

.PHONY: test
test: envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" ginkgo -r -coverprofile cover.out

.PHONY: docker-build
docker-build:
	docker build -t ${IMAGE} .

vendir: bin/vendir
	vendir sync --chdir tests


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

bin/vendir:
	go install carvel.dev/vendir/cmd/vendir@latest
