.ONESHELL:
CGO_ENABLED := $(or ${CGO_ENABLED},0)
GO := go
GO111MODULE := on

release:: generate-client gofmt test;

.PHONY: gofmt
gofmt:
	GO111MODULE=off $(GO) fmt ./...

.PHONY: test
test:
	CGO_ENABLED=1 $(GO) test ./... -coverprofile=coverage.out -covermode=atomic && go tool cover -func=coverage.out

.PHONY: generate-client
generate-client:
	rm -rf api
	mkdir -p api
	GO111MODULE=off swagger generate client -f metal-api.json -t api --skip-validation

.PHONY: golangcicheck
golangcicheck:
	@/bin/bash -c "type -P golangci-lint;" 2>/dev/null || (echo "golangci-lint is required but not available in current PATH. Install: https://github.com/golangci/golangci-lint#install"; exit 1)

.PHONY: lint
lint: golangcicheck
	CGO_ENABLED=1 golangci-lint run