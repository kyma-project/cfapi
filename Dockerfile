# Build the manager binary
FROM golang:1.22.1-alpine as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# Copy the go source
COPY main.go main.go
COPY api api/
COPY controllers controllers/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

RUN GO111MODULE=on go get github.com/mikefarah/yq/v3
RUN apk add curl

ARG TAG_default_tag=from_dockerfile

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-X 'main.buildVersion=${TAG_default_tag}'" -a -o manager main.go


ENV VERSION_SERVICEBINDING=0.4.0
ENV VERSION_KPACK=0.13.4
ENV VERSION_CERT_MANAGER=1.14.6
ENV VERSION_GATEWAY_API=1.1.0
ENV VERSION_TWUNI=2.2.3
ENV VERSION_KORIFI=0.12.0


WORKDIR /workspace/module-data/servicebinding
RUN curl -OLf https://github.com/servicebinding/runtime/releases/download/v$VERSION_SERVICEBINDING/servicebinding-runtime-v$VERSION_SERVICEBINDING.yaml
RUN curl -OLf https://github.com/servicebinding/runtime/releases/download/v$VERSION_SERVICEBINDING/servicebinding-workloadresourcemappings-v$VERSION_SERVICEBINDING.yaml 

WORKDIR /workspace/module-data/kpack
RUN curl -OLf https://github.com/buildpacks-community/kpack/releases/download/v$VERSION_KPACK/release-$VERSION_KPACK.yaml

WORKDIR /workspace/module-data/cert-manager
RUN curl -OLf https://github.com/cert-manager/cert-manager/releases/download/v$VERSION_CERT_MANAGER/cert-manager.yaml

WORKDIR /workspace/module-data/gateway-api
RUN curl -OLf https://github.com/kubernetes-sigs/gateway-api/releases/download/v$VERSION_GATEWAY_API/experimental-install.yaml

WORKDIR /workspace/module-data/twuni-helm
RUN curl -OLf https://github.com/twuni/docker-registry.helm/archive/refs/tags/v$VERSION_TWUNI.tar.gz

#Some day we are going to use the OSS Korifi project
#WORKDIR /workspace/module-data/korifi
#RUN curl -OLf https://github.com/cloudfoundry/korifi/releases/download/v$VERSION_KORIFI/korifi-$VERSION_KORIFI.tgz

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder --chown=65532:65532  /workspace/module-data module-data/
COPY --chown=65532:65532 module-data module-data/ 
USER 65532:65532

ENTRYPOINT ["/manager"]
