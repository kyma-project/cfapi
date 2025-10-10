ARG REGISTRY=my-registry
ARG IMG=my-img
ARG VERSION=my-version

#-------
FROM ${REGISTRY}/${IMG}:${VERSION} AS uploader
ARG VERSION

WORKDIR /workspace
COPY release/$VERSION/korifi-${VERSION} korifi-chart
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

COPY --from=uploader /workspace/korifi-chart /korifi-chart
# If the uploader image contains a non-empty korifi directory (i.e. a local
# build has been performed), replace the korifi helm chart with the custom
# built one
RUN if [ $(ls -A "/korifi-chart" | wc -l) -ne 0 ]; then \
        echo "Korifi files found, replacing Korifi module"; \
        rm -rf /module-data/korifi-chart/* && cp -a /korifi-chart/* /module-data/korifi-chart/; \
    fi

#--------
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=replacer /manager .
COPY --chown=65532:65532 /module-data .
COPY --from=replacer --chown=65532:65532  /module-data module-data/

USER 65532:65532

ENTRYPOINT ["/manager"]
