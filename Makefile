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

IMAGE_REPOSITORY := eu.gcr.io/gardener-project/gardener/machine-controller-manager
IMAGE_TAG        := $(shell cat VERSION)

CONTROL_NAMESPACE  := default
CONTROL_KUBECONFIG := dev/target-kubeconfig.yaml
TARGET_KUBECONFIG  := dev/target-kubeconfig.yaml

#########################################
# Rules for local development scenarios #
#########################################

.PHONY: start
start:
	@go run cmd/machine-controller-manager/controller_manager.go \
			--control-kubeconfig=$(CONTROL_KUBECONFIG) \
			--target-kubeconfig=$(TARGET_KUBECONFIG) \
			--namespace=$(CONTROL_NAMESPACE) \
			--safety-up=2 \
			--safety-down=1 \
			--machine-drain-timeout=5 \
			--machine-health-timeout=10 \
			--machine-set-scale-timeout=20 \
			--machine-safety-orphan-vms-period=30 \
			--machine-safety-overshooting-period=1 \
			--v=2

#################################################################
# Rules related to binary build, Docker image build and release #
#################################################################

.PHONY: revendor
revendor:
	@dep ensure -update

.PHONY: build
build:
	@.ci/build

.PHONY: build-local
build-local:
	@env LOCAL_BUILD=1 .ci/build

.PHONY: release
release: build build-local docker-image docker-login docker-push rename-binaries

.PHONY: docker-image
docker-images:
	@if [[ ! -f bin/rel/machine-controller-manager ]]; then echo "No binary found. Please run 'make build'"; false; fi
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) --rm .

.PHONY: docker-login
docker-login:
	@gcloud auth activate-service-account --key-file .kube-secrets/gcr/gcr-readwrite.json

.PHONY: docker-push
docker-push:
	@if ! docker images $(IMAGE_REPOSITORY) | awk '{ print $$2 }' | grep -q -F $(IMAGE_TAG); then echo "$(IMAGE_REPOSITORY) version $(IMAGE_TAG) is not yet built. Please run 'make docker-images'"; false; fi
	@gcloud docker -- push $(IMAGE_REPOSITORY):$(IMAGE_TAG)

.PHONY: rename-binaries
rename-binaries:
	@if [[ -f bin/machine-controller-manager ]]; then cp bin/machine-controller-manager machine-controller-manager-darwin-amd64; fi
	@if [[ -f bin/rel/machine-controller-manager ]]; then cp bin/rel/machine-controller-manager machine-controller-manager-linux-amd64; fi

.PHONY: clean
clean:
	@rm -rf bin/
	@rm -f *linux-amd64
	@rm -f *darwin-amd64

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

.PHONY: test-cov
test-cov:
	@env COVERAGE=1 .ci/test
	@echo "mode: set" > machine-controller-manager.coverprofile && find . -name "*.coverprofile" -type f | xargs cat | grep -v mode: | sort -r | awk '{if($$1 != last) {print $$0;last=$$1}}' >> machine-controller-manager.coverprofile
	@go tool cover -html=machine-controller-manager.coverprofile -o=machine-controller-manager.coverage.html
	@rm machine-controller-manager.coverprofile

.PHONY: test-clean
test-clean:
	@find . -name "*.coverprofile" -type f -delete
	@rm -f machine-controller-manager.coverage.html




### Adopted from cluster-api ###

.PHONY: genapi genconversion genclientset gendeepcopy

all: generate build images

depend:
	dep version || go get -u github.com/golang/dep/cmd/dep
	dep ensure -v

	# go libraries often ship BUILD and BUILD.bazel files, but they often don't work.
	# We delete them and regenerate them
	find vendor -name "BUILD" -delete
	find vendor -name "BUILD.bazel" -delete

	bazel run //:gazelle

generate: genapi genconversion genclientset gendeepcopy genopenapi

genapi: depend
	go build -o $$GOPATH/bin/apiregister-gen sigs.k8s.io/cluster-api/vendor/github.com/kubernetes-incubator/apiserver-builder/cmd/apiregister-gen
	apiregister-gen -i ./pkg/apis,./pkg/apis/cluster,./pkg/apis/cluster/v1alpha1

genconversion: depend
	go build -o $$GOPATH/bin/conversion-gen sigs.k8s.io/cluster-api/vendor/k8s.io/code-generator/cmd/conversion-gen
	conversion-gen -i ./pkg/apis/cluster/v1alpha1/ -O zz_generated.conversion --go-header-file boilerplate.go.txt

genclientset: depend
	go build -o $$GOPATH/bin/client-gen sigs.k8s.io/cluster-api/vendor/k8s.io/code-generator/cmd/client-gen
	client-gen \
	  --input="cluster/v1alpha1" \
		--clientset-name="clientset" \
		--input-base="sigs.k8s.io/cluster-api/pkg/apis" \
		--output-package "sigs.k8s.io/cluster-api/pkg/client/clientset_generated" \
		--go-header-file boilerplate.go.txt \
		--clientset-path sigs.k8s.io/cluster-api/pkg/client/clientset_generated

gendeepcopy:
	go build -o $$GOPATH/bin/deepcopy-gen sigs.k8s.io/cluster-api/vendor/k8s.io/code-generator/cmd/deepcopy-gen
	deepcopy-gen \
	  -i ./pkg/apis/cluster/,./pkg/apis/cluster/v1alpha1/ \
	  -O zz_generated.deepcopy \
	  -h boilerplate.go.txt

STATIC_API_DIRS = k8s.io/apimachinery/pkg/apis/meta/v1
STATIC_API_DIRS += k8s.io/apimachinery/pkg/api/resource
STATIC_API_DIRS += k8s.io/apimachinery/pkg/version
STATIC_API_DIRS += k8s.io/apimachinery/pkg/runtime
STATIC_API_DIRS += k8s.io/apimachinery/pkg/util/intstr
STATIC_API_DIRS += k8s.io/api/core/v1

# Automatically extract vendored apis under vendor/k8s.io/api.
VENDOR_API_DIRS := $(shell find vendor/k8s.io/api -type d | grep -E 'v[[:digit:]]+(alpha[[:digit:]]+|beta[[:digit:]]+)*' | sed -e 's/^vendor\///')

empty:=
comma:=,
space:=$(empty) $(empty)

genopenapi: static_apis = $(subst $(space),$(comma),$(STATIC_API_DIRS))
genopenapi: vendor_apis = $(subst $(space),$(comma),$(VENDOR_API_DIRS))

genopenapi:
	go build -o $$GOPATH/bin/openapi-gen sigs.k8s.io/cluster-api/vendor/k8s.io/code-generator/cmd/openapi-gen
	openapi-gen \
	  --input-dirs $(static_apis) \
	  --input-dirs $(vendor_apis) \
	  --input-dirs ./pkg/apis/cluster/,./pkg/apis/cluster/v1alpha1/ \
	  --output-package "sigs.k8s.io/cluster-api/pkg/openapi" \
	  --go-header-file boilerplate.go.txt

build: depend
	CGO_ENABLED=0 go install -a -ldflags '-extldflags "-static"' sigs.k8s.io/cluster-api/cmd/apiserver
	CGO_ENABLED=0 go install -a -ldflags '-extldflags "-static"' sigs.k8s.io/cluster-api/cmd/controller-manager

images: depend
	$(MAKE) -C cmd/apiserver image
	$(MAKE) -C cmd/controller-manager image

verify:
	./hack/verify_boilerplate.py