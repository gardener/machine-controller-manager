#############      builder                                  #############
FROM golang:1.22.5 AS builder

WORKDIR /go/src/github.com/gardener/machine-controller-manager
COPY . .

RUN .ci/build

#############      base                                     #############
FROM gcr.io/distroless/static-debian11:nonroot as base
WORKDIR /

#############      machine-controller-manager               #############
FROM base AS machine-controller-manager

COPY --from=builder /go/src/github.com/gardener/machine-controller-manager/bin/machine-controller-manager /machine-controller-manager
ENTRYPOINT ["/machine-controller-manager"]
