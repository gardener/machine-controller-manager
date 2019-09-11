FROM alpine:3.10

RUN apk add --update bash curl tzdata

COPY bin/rel/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
