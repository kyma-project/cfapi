ARG REGISTRY=my-registry
ARG IMG=my-img
ARG VERSION=my-version

#-------
FROM ${REGISTRY}/${IMG}:${VERSION} as uploader
ARG REGISTRY
ARG IMG
ARG VERSION

WORKDIR /workspace
COPY development/release-output/korifi-${VERSION}.tgz korifi-${VERSION}.tgz

#--------
FROM golang:1.24-alpine as yq-image
ARG REGISTRY
ARG IMG
ARG VERSION

RUN go install github.com/mikefarah/yq/v4@latest


#--------
FROM yq-image as replacer
ARG REGISTRY
ARG IMG
ARG VERSION

WORKDIR /
COPY --from=uploader /manager .

COPY --chown=65532:65532 /module-data .
COPY --from=uploader --chown=65532:65532  /module-data module-data/
RUN rm -rf /module-data/korifi/*.tgz

RUN yq -i eval '.spec.isCA = true'  module-data/certificates/certificates.tmpl

COPY --from=uploader /workspace/korifi-${VERSION}.tgz /module-data/korifi/korifi-${VERSION}.tgz

#--------
FROM gcr.io/distroless/static:nonroot
ARG REGISTRY
ARG IMG
ARG VERSION

WORKDIR /
COPY --from=replacer /manager .
COPY --chown=65532:65532 /module-data .
COPY --from=replacer --chown=65532:65532  /module-data module-data/

USER 65532:65532

ENTRYPOINT ["/manager"]
