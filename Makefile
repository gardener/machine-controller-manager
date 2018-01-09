VCS              := github.com
ORGANIZATION     := gardener
PROJECT          := node-controller-manager
REPOSITORY       := $(VCS)/$(ORGANIZATION)/$(PROJECT)
VERSION          := v0.1.0
PACKAGES         := $(shell go list ./... | grep -vE '/vendor/|/pkg/test|/code-generator|/pkg/client|/pkg/openapi')
LINT_FOLDERS     := $(shell echo $(PACKAGES) | sed "s|$(REPOSITORY)|.|g")
BINARY_PATH      := $(REPOSITORY)/cmd/$(PROJECT)

IMAGE_REPOSITORY := kvmprashanth/node-controller-manager
IMAGE_TAG        := v2

TYPES_FILES      := $(shell find pkg/apis -name types.go)

BINDIR           := bin
GOBIN            := $(PWD)/bin
PATH             := $(GOBIN):$(PATH)
USER             := $(shell id -u -n)

export PATH
export GOBIN

.PHONY: dev
dev: 
	@go run cmd/node-controller-manager/controller_manager.go --kubeconfig=dev/kubeconfig.yaml --v=2

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
	@dep ensure -update
	@dep prune

.PHONY: generate-files
generate-files: .generate_files

# Regenerate all files if the gen exes changed or any "types.go" files changed
.generate_files: .generate_exes $(TYPES_FILES)

	# Generate defaults
	$(BINDIR)/defaulter-gen \
		--v 1 --logtostderr \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
		--extra-peer-dirs "$(REPOSITORY)/pkg/apis/machine" \
		--extra-peer-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
		--output-file-base "zz_generated.defaults"
	# Generate deep copies
	$(BINDIR)/deepcopy-gen \
		--v 1 --logtostderr \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
		--bounding-dirs "$(REPOSITORY)" \
		--output-file-base zz_generated.deepcopy
	# Generate conversions
	$(BINDIR)/conversion-gen \
		--v 1 --logtostderr \
		--extra-peer-dirs k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/conversion,k8s.io/apimachinery/pkg/runtime \
	  --go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
		--output-file-base zz_generated.conversion
	# generate openapi
	$(BINDIR)/openapi-gen \
		--v 1 --logtostderr \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1,k8s.io/api/core/v1,k8s.io/apimachinery/pkg/apis/meta/v1" \
		--output-package "$(REPOSITORY)/pkg/openapi"

	# Generate the internal clientset (pkg/client/internalclientset)
	${BINDIR}/client-gen "$@" \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-base "$(REPOSITORY)/pkg/apis/" \
		--input machine \
		--clientset-path "$(REPOSITORY)/pkg/client" \
		--clientset-name internalclientset \
	# Generate the versioned clientset (pkg/client/clientset/clientset)
	${BINDIR}/client-gen "$@" \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-base "$(REPOSITORY)/pkg/apis/" \
		--input "machine/v1alpha1" \
		--clientset-path "$(REPOSITORY)/pkg/client" \
		--clientset-name "clientset" \
	# generate lister
	${BINDIR}/lister-gen "$@" \
		--go-header-file "verify/boilerplate/boilerplate.go.txt" \
		--input-dirs="$(REPOSITORY)/pkg/apis/machine" \
		--input-dirs="$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
		--output-package "$(REPOSITORY)/pkg/client/listers" \
	# generate informer
	${BINDIR}/informer-gen "$@" \
	--go-header-file "verify/boilerplate/boilerplate.go.txt" \
	--input-dirs "$(REPOSITORY)/pkg/apis/machine" \
	--input-dirs "$(REPOSITORY)/pkg/apis/machine/v1alpha1" \
	--versioned-clientset-package "$(REPOSITORY)/pkg/client/clientset" \
	--internal-clientset-package "$(REPOSITORY)/pkg/client/internalclientset" \
	--listers-package "$(REPOSITORY)/pkg/client/listers" \
	--output-package "$(REPOSITORY)/pkg/client/informers"
	# --single-directory \

# This section contains the code generation stuff
#################################################
.generate_exes: $(BINDIR)/defaulter-gen \
                $(BINDIR)/deepcopy-gen \
                $(BINDIR)/conversion-gen \
                $(BINDIR)/client-gen \
                $(BINDIR)/lister-gen \
                $(BINDIR)/informer-gen \
                $(BINDIR)/openapi-gen

$(BINDIR)/defaulter-gen:
	@go build -o $@ code-generator/defaulter-gen/main.go

$(BINDIR)/deepcopy-gen:
	@go build -o $@ code-generator/deepcopy-gen/main.go

$(BINDIR)/conversion-gen:
	@go build -o $@ code-generator/conversion-gen/main.go

$(BINDIR)/client-gen:
	@go build -o $@ code-generator/client-gen/main.go

$(BINDIR)/lister-gen:
	@go build -o $@ code-generator/lister-gen/main.go

$(BINDIR)/informer-gen:
	@go build -o $@ code-generator/informer-gen/main.go

$(BINDIR)/openapi-gen:
	@go build -o $@ code-generator/openapi-gen/main.go