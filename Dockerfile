FROM alpine:3.6

RUN apk add --update bash curl

COPY ./bin/node-controller-manager /node-controller-manager
WORKDIR /
ENTRYPOINT ["/node-controller-manager"]
