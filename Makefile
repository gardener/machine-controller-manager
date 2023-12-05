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
			-mod=vendor \
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

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod tidy -v
	@GO111MODULE=on go mod vendor -v

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

generate: controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/apis/machine/v1alpha1/... output:crd:dir=kubernetes/crds output:stdout
	@./hack/generate-code
	@./hack/api-reference/generate-spec-doc.sh

# find or download controller-gen
# download controller-gen if necessary
.PHONY: controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

.PHONY: add-license-headers
add-license-headers: $(GO_ADD_LICENSE)
	@./hack/add_license_headers.sh
