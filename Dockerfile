FROM alpine:3.6

RUN apk add --update bash curl

COPY ./bin/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
