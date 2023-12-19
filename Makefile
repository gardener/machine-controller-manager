# Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

-include .env
include hack/tools.mk

IMAGE_REPOSITORY   := europe-docker.pkg.dev/gardener-project/public/gardener/machine-controller-manager
IMAGE_TAG          := $(shell cat VERSION)
COVERPROFILE       := test/output/coverprofile.out

LEADER_ELECT 	   := "true"
MACHINE_SAFETY_OVERSHOOTING_PERIOD:=1m

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

###########################################
# Setup targets for gardener shoot 		  #
###########################################

.PHONY: gardener-setup
gardener-setup:
	@echo "enter project name"; \
	read PROJECT; \
	echo "enter seed name"; \
	read SEED; \
	echo "enter shoot name"; \
	read SHOOT; \
	echo "enter cluster provider(gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)"; \
	read PROVIDER; \
	./hack/gardener_local_setup.sh --seed $$SEED --shoot $$SHOOT --project $$PROJECT --provider $$PROVIDER

.PHONY: gardener-local-mcm-up
gardener-local-mcm-up: gardener-setup
	$(MAKE) start;

.PHONY: gardener-restore
gardener-restore:
	@echo "enter project name"; \
	read PROJECT; \
	echo "enter shoot name"; \
	read SHOOT; \
	echo "enter cluster provider(gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)"; \
	read PROVIDER; \
	./hack/gardener_local_restore.sh --shoot $$SHOOT --project $$PROJECT --provider $$PROVIDER


###########################################
# Setup targets for non-gardener          #
###########################################

.PHONY: non-gardener-setup
non-gardener-setup:
	@echo "enter namespace"; \
	read NAMESPACE; \
	echo "enter control kubeconfig path"; \
	read CONTROL_KUBECONFIG_PATH; \
	echo "enter target kubeconfig path"; \
	read TARGET_KUBECONFIG_PATH; \
	echo "enter cluster provider(gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)"; \
	read PROVIDER; \
	./hack/non_gardener_local_setup.sh --namespace $$NAMESPACE --control-kubeconfig-path $$CONTROL_KUBECONFIG_PATH --target-kubeconfig-path $$TARGET_KUBECONFIG_PATH --provider $$PROVIDER

.PHONY: non-gardener-local-mcm-up
non-gardener-local-mcm-up: non-gardener-setup
	$(MAKE) start;

.PHONY: non-gardener-restore
non-gardener-restore:
	@echo "enter namespace"; \
	read NAMESPACE; \
	echo "enter control kubeconfig path"; \
	read CONTROL_KUBECONFIG_PATH; \
	echo "enter cluster provider(gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)"; \
	read PROVIDER; \
	@echo "enter project name"; \
	./hack/non_gardener_local_restore.sh --namespace $$NAMESPACE --control-kubeconfig-path $$CONTROL_KUBECONFIG_PATH --provider $$PROVIDER

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start
start:
	@GO111MODULE=on go run \
			cmd/machine-controller-manager/controller_manager.go \
			--control-kubeconfig=${CONTROL_KUBECONFIG} \
			--target-kubeconfig=${TARGET_KUBECONFIG} \
			--namespace=${CONTROL_NAMESPACE} \
			--safety-up=2 \
			--safety-down=1 \
			--machine-safety-overshooting-period=$(MACHINE_SAFETY_OVERSHOOTING_PERIOD) \
			--leader-elect=$(LEADER_ELECT) \
			--v=3

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: tidy
tidy:
	@GO111MODULE=on go mod tidy

.PHONY: build
build:
	@.ci/build

.PHONY: release
release: build docker-image docker-login docker-push

.PHONY: docker-image
docker-image:
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) --rm .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: clean
clean:
	@rm -rf bin/

#####################################################################
# Rules for verification, formatting, linting, testing and cleaning #
#####################################################################

.PHONY: verify
verify: check test

.PHONY: check
check:
	@.ci/check

.PHONY: test
test:
	@.ci/test

.PHONY: test-unit
test-unit:
	@SKIP_INTEGRATION_TESTS=X .ci/test

.PHONY: test-integration
test-integration:
	@SKIP_UNIT_TESTS=X .ci/test

.PHONY: show-coverage
show-coverage:
	@if [ ! -f $(COVERPROFILE) ]; then echo "$(COVERPROFILE) is not yet built. Please run 'COVER=true make test'"; false; fi
	go tool cover -html $(COVERPROFILE)

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f $(COVERPROFILE)

##@ Deployment

.PHONY: fmt
fmt: goimports ## Run goimports against code.
	$(GOIMPORTS) -w .

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: lint
lint: golangci-lint ## Run golangci-lint on the code.
	$(GOLANGCI_LINT) run ./...

.PHONY: add-license
add-license: addlicense ## Add license headers to all go files.
	find . -name '*.go' -exec $(ADDLICENSE) -f hack/LICENSE_BOILERPLATE.txt {} +

.PHONY: check-license
check-license: addlicense ## Check that every file has a license header present.
	find . -name '*.go' -exec $(ADDLICENSE) -check -c 'SAP SE or an SAP affiliate company' {} +

.PHONY: generate
generate: vgopath deepcopy-gen defaulter-gen conversion-gen openapi-gen controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/apis/machine/v1alpha1/... output:crd:dir=kubernetes/crds output:stdout
	VGOPATH=$(VGOPATH) \
	DEEPCOPY_GEN=$(DEEPCOPY_GEN) \
	DEFAULTER_GEN=$(DEFAULTER_GEN) \
	CONVERSION_GEN=$(CONVERSION_GEN) \
	OPENAPI_GEN=$(OPENAPI_GEN) \
	./hack/update-codegen.sh

##@ Tools

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
DEEPCOPY_GEN ?= $(LOCALBIN)/deepcopy-gen
DEFAULTER_GEN ?= $(LOCALBIN)/defaulter-gen
CONVERSION_GEN ?= $(LOCALBIN)/conversion-gen
OPENAPI_GEN ?= $(LOCALBIN)/openapi-gen
VGOPATH ?= $(LOCALBIN)/vgopath
GEN_CRD_API_REFERENCE_DOCS ?= $(LOCALBIN)/gen-crd-api-reference-docs
ADDLICENSE ?= $(LOCALBIN)/addlicense
GOIMPORTS ?= $(LOCALBIN)/goimports
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
CODE_GENERATOR_VERSION ?= v0.28.2
VGOPATH_VERSION ?= v0.1.3
CONTROLLER_TOOLS_VERSION ?= v0.13.0
GEN_CRD_API_REFERENCE_DOCS_VERSION ?= v0.3.0
ADDLICENSE_VERSION ?= v1.1.1
GOIMPORTS_VERSION ?= v0.13.0
GOLANGCI_LINT_VERSION ?= v1.55.2

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: deepcopy-gen
deepcopy-gen: $(DEEPCOPY_GEN) ## Download deepcopy-gen locally if necessary.
$(DEEPCOPY_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/deepcopy-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/deepcopy-gen@$(CODE_GENERATOR_VERSION)

.PHONY: defaulter-gen
defaulter-gen: $(DEFAULTER_GEN) ## Download defaulter-gen locally if necessary.
$(DEFAULTER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/defaulter-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/defaulter-gen@$(CODE_GENERATOR_VERSION)

.PHONY: conversion-gen
conversion-gen: $(CONVERSION_GEN) ## Download conversion-gen locally if necessary.
$(CONVERSION_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/conversion-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/conversion-gen@$(CODE_GENERATOR_VERSION)

.PHONY: openapi-gen
openapi-gen: $(OPENAPI_GEN) ## Download openapi-gen locally if necessary.
$(OPENAPI_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/openapi-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/openapi-gen@$(CODE_GENERATOR_VERSION)

.PHONY: vgopath
vgopath: $(VGOPATH) ## Download vgopath locally if necessary.
.PHONY: $(VGOPATH)
$(VGOPATH): $(LOCALBIN)
	@if test -x $(LOCALBIN)/vgopath && ! $(LOCALBIN)/vgopath version | grep -q $(VGOPATH_VERSION); then \
		echo "$(LOCALBIN)/vgopath version is not expected $(VGOPATH_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/vgopath; \
	fi
	test -s $(LOCALBIN)/vgopath || GOBIN=$(LOCALBIN) go install github.com/ironcore-dev/vgopath@$(VGOPATH_VERSION)

.PHONY: gen-crd-api-reference-docs
gen-crd-api-reference-docs: $(GEN_CRD_API_REFERENCE_DOCS) ## Download gen-crd-api-reference-docs locally if necessary.
$(GEN_CRD_API_REFERENCE_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/gen-crd-api-reference-docs || GOBIN=$(LOCALBIN) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

.PHONY: addlicense
addlicense: $(ADDLICENSE) ## Download addlicense locally if necessary.
$(ADDLICENSE): $(LOCALBIN)
	test -s $(LOCALBIN)/addlicense || GOBIN=$(LOCALBIN) go install github.com/google/addlicense@$(ADDLICENSE_VERSION)

.PHONY: goimports
goimports: $(GOIMPORTS) ## Download goimports locally if necessary.
$(GOIMPORTS): $(LOCALBIN)
	test -s $(LOCALBIN)/goimports || GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
