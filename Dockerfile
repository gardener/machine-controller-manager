FROM alpine:3.6

RUN apk add --update bash curl

# Resources used by Golang libary
ENV ZONEINFO=/zone-info/zoneinfo.zip
ADD ./assets/zoneinfo.zip /zone-info/zoneinfo.zip

COPY bin/rel/machine-controller-manager /machine-controller-manager
WORKDIR /
ENTRYPOINT ["/machine-controller-manager"]
