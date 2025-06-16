#############      builder                                  #############
FROM golang:1.23.3 AS builder

WORKDIR /go/src/github.com/gardener/machine-controller-manager
COPY . .

RUN --mount=type=cache,target="/root/.cache/go-build" .ci/build

#############      base                                     #############
FROM gcr.io/distroless/static-debian12:nonroot as base
WORKDIR /

#############      machine-controller-manager               #############
FROM base AS machine-controller-manager

COPY --from=builder /go/src/github.com/gardener/machine-controller-manager/bin/machine-controller-manager /machine-controller-manager
ENTRYPOINT ["/machine-controller-manager"]
