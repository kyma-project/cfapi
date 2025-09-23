ARG REGISTRY=my-registry
ARG IMG=my-img
ARG VERSION=my-version

#-------
FROM ${REGISTRY}/${IMG}:${VERSION} AS uploader
ARG VERSION

WORKDIR /workspace
# Copying the korifi tgz file via a wild card would not fail if the tgz does
# not exist. We use this trick to support either using korifi from the uploader
# (i.e. official Korifi release), or building a local dev Korifi version. This
# Dockerfile would only replace Korifi if the tgz below exists
COPY release/$VERSION/korifi-${VERSION}.tgz* /workspace
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

RUN yq -i eval '.spec.isCA = true'  module-data/certificates/certificates.tmpl

RUN rm -rf /module-data/btp-service-broker
COPY --from=uploader /workspace/btp-service-broker /module-data/btp-service-broker

COPY --from=uploader /workspace/korifi-${VERSION}.tgz* /
RUN test -f /korifi-${VERSION}.tgz && rm -rf /module-data/korifi/*.tgz && cp /korifi-${VERSION}.tgz /module-data/korifi/korifi-${VERSION}.tgz || echo "Using pre-packaged Korifi"

#--------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=replacer /manager .
COPY --chown=65532:65532 /module-data .
COPY --from=replacer --chown=65532:65532  /module-data module-data/

USER 65532:65532

ENTRYPOINT ["/manager"]
