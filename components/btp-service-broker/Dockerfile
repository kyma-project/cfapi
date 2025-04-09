# syntax = docker/dockerfile:experimental
FROM golang:1.22.5 as builder

WORKDIR /workspace

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY main.go main.go
COPY config config
COPY osbapi osbapi
COPY handlers handlers
COPY middleware middleware
COPY routing routing
COPY service_manager service_manager
COPY tools tools

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o btp-service-broker main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/btp-service-broker .
USER 1000:1000

ENTRYPOINT [ "/btp-service-broker" ]
