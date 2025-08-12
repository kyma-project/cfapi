ARG REGISTRY=my-registry
ARG IMG=my-img
ARG VERSION=my-version

#-------
FROM ${REGISTRY}/${IMG}:${VERSION} AS uploader
ARG VERSION

WORKDIR /workspace
COPY release/$VERSION/korifi-${VERSION}.tgz korifi-${VERSION}.tgz
COPY release/$VERSION/btp-service-broker btp-service-broker

#--------
FROM golang:1.24-alpine AS yq-image

RUN go install github.com/mikefarah/yq/v4@latest


#--------
FROM yq-image AS replacer
ARG VERSION

WORKDIR /
COPY --from=uploader /manager .

COPY --chown=65532:65532 /module-data .
COPY --from=uploader --chown=65532:65532  /module-data module-data/
RUN rm -rf /module-data/korifi/*.tgz
RUN rm -rf /module-data/btp-service-broker

RUN yq -i eval '.spec.isCA = true'  module-data/certificates/certificates.tmpl

COPY --from=uploader /workspace/korifi-${VERSION}.tgz /module-data/korifi/korifi-${VERSION}.tgz
COPY --from=uploader /workspace/btp-service-broker /module-data/btp-service-broker

#--------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=replacer /manager .
COPY --chown=65532:65532 /module-data .
COPY --from=replacer --chown=65532:65532  /module-data module-data/

USER 65532:65532

ENTRYPOINT ["/manager"]
