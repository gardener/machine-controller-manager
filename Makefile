# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

-include .env
TOOLS_DIR := hack/tools
include hack/tools.mk

REPOSITORY         := $(shell go list -m)
IMAGE_REPOSITORY   := europe-docker.pkg.dev/gardener-project/public/gardener/machine-controller-manager
VERSION            := $(shell cat VERSION)
IMAGE_TAG          := $(VERSION)
GIT_SHA            := $(shell git rev-parse --short HEAD || echo "GitNotFound")
COVERPROFILE       := test/output/coverprofile.out

LEADER_ELECT 	   ?= "true" # If LEADER_ELECT is not set in the environment, use the default value "true"
MACHINE_SAFETY_OVERSHOOTING_PERIOD:=1m

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

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
			-ldflags "-w -X ${REPOSITORY}/pkg/version.Version=${VERSION} -X ${REPOSITORY}/pkg/version.GitSHA=${GIT_SHA}" \
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
	@GO111MODULE=on go mod tidy -v

.PHONY: build
build:
	@.ci/build

.PHONY: release
release: build docker-image docker-login docker-push

PLATFORM ?= linux/amd64
.PHONY: docker-image
docker-image:
	@docker buildx build --platform $(PLATFORM)  -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) --rm .

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

.PHONY: generate
generate: $(VGOPATH) $(DEEPCOPY_GEN) $(DEFAULTER_GEN) $(CONVERSION_GEN) $(OPENAPI_GEN) $(CONTROLLER_GEN) $(GEN_CRD_API_REFERENCE_DOCS)
	$(CONTROLLER_GEN) crd paths=./pkg/apis/machine/v1alpha1/... output:crd:dir=kubernetes/crds output:stdout
	@./hack/generate-code
	@./hack/api-reference/generate-spec-doc.sh

.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@./hack/add_license_headers.sh

.PHONY: sast
sast: $(GOSEC)
	@./hack/sast.sh

.PHONY: sast-report
sast-report:$(GOSEC)
	@./hack/sast.sh --gosec-report true
