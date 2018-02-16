VCS              	:= github.com
ORGANIZATION     	:= gardener
PROJECT          	:= machine-controller-manager
REPOSITORY       	:= $(VCS)/$(ORGANIZATION)/$(PROJECT)
VERSION          	:= 0.1.0
PACKAGES         	:= $(shell go list ./... | grep -vE '/vendor/|/pkg/test|/code-generator|/pkg/client|/pkg/openapi')
LINT_FOLDERS     	:= $(shell echo $(PACKAGES) | sed "s|$(REPOSITORY)|.|g")
BINARY_PATH      	:= $(REPOSITORY)/cmd/$(PROJECT)

IMAGE_REPOSITORY 	:= eu.gcr.io/gardener-project/gardener/machine-controller-manager
IMAGE_TAG        	:= 0.1.0

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
	@go run cmd/machine-controller-manager/controller_manager.go --control-kubeconfig=$(CONTROL_KUBECONFIG) --target-kubeconfig=$(TARGET_KUBECONFIG) --namespace=$(CONTROL_NAMESPACE) --v=2

.PHONY: build
build:
	@env GOOS=linux GOARCH=amd64 go build -o $(BINDIR)/machine-controller-manager cmd/machine-controller-manager/controller_manager.go

.PHONY: docker-image
docker-image:
	@if [[ ! -f bin/machine-controller-manager ]]; then echo "No binary found. Please run 'make docker-build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) --rm .

.PHONY: push
push:
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: revendor
revendor:
	@dep ensure
	@dep prune
