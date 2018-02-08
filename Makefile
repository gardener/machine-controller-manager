VCS              	:= github.com
ORGANIZATION     	:= gardener
PROJECT          	:= node-controller-manager
REPOSITORY       	:= $(VCS)/$(ORGANIZATION)/$(PROJECT)
VERSION          	:= v0.1.0
PACKAGES         	:= $(shell go list ./... | grep -vE '/vendor/|/pkg/test|/code-generator|/pkg/client|/pkg/openapi')
LINT_FOLDERS     	:= $(shell echo $(PACKAGES) | sed "s|$(REPOSITORY)|.|g")
BINARY_PATH      	:= $(REPOSITORY)/cmd/$(PROJECT)

IMAGE_REPOSITORY 	:= kvmprashanth/node-controller-manager
IMAGE_TAG        	:= v0.1.2

TYPES_FILES      	:= $(shell find pkg/apis -name types.go)

BINDIR           	:= bin
GOBIN            	:= $(PWD)/bin
PATH             	:= $(GOBIN):$(PATH)
USER             	:= $(shell id -u -n)

CONTROL_NAMESPACE 	:= default
CONTROL_KUBECONFIG 	:= dev/target-kubeconfig.yaml
TARGET_KUBECONFIG 	:= dev/target-kubeconfig.yaml

export PATH
export GOBIN

.PHONY: dev
dev:
	@go run cmd/node-controller-manager/controller_manager.go --control-kubeconfig=$(CONTROL_KUBECONFIG) --target-kubeconfig=$(TARGET_KUBECONFIG) --namespace=$(CONTROL_NAMESPACE) --v=2

.PHONY: build
build:
	@env GOOS=linux GOARCH=amd64 go build -o $(BINDIR)/node-controller-manager cmd/node-controller-manager/controller_manager.go

.PHONY: docker-image
docker-image:
	@if [[ ! -f bin/node-controller-manager ]]; then echo "No binary found. Please run 'make docker-build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) --rm .

.PHONY: push
push:
	@docker push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: revendor
revendor:
	@dep ensure
	@dep prune
