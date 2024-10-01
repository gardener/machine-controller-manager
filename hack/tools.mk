# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

TOOLS_DIR                  := hack/tools
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin

## Tool Binaries
CONTROLLER_GEN ?= $(TOOLS_BIN_DIR)/controller-gen
DEEPCOPY_GEN ?= $(TOOLS_BIN_DIR)/deepcopy-gen
DEFAULTER_GEN ?= $(TOOLS_BIN_DIR)/defaulter-gen
CONVERSION_GEN ?= $(TOOLS_BIN_DIR)/conversion-gen
OPENAPI_GEN ?= $(TOOLS_BIN_DIR)/openapi-gen
VGOPATH ?= $(TOOLS_BIN_DIR)/vgopath
GEN_CRD_API_REFERENCE_DOCS ?= $(TOOLS_BIN_DIR)/gen-crd-api-reference-docs
ADDLICENSE ?= $(TOOLS_BIN_DIR)/addlicense
GOIMPORTS ?= $(TOOLS_BIN_DIR)/goimports
GOLANGCI_LINT ?= $(TOOLS_BIN_DIR)/golangci-lint

## Tool Versions
CODE_GENERATOR_VERSION ?= v0.31.0
VGOPATH_VERSION ?= v0.1.6
CONTROLLER_TOOLS_VERSION ?= v0.16.1
GEN_CRD_API_REFERENCE_DOCS_VERSION ?= v0.3.0
ADDLICENSE_VERSION ?= v1.1.1
GOIMPORTS_VERSION ?= v0.13.0
GOLANGCI_LINT_VERSION ?= v1.60.3


# default tool versions
GO_ADD_LICENSE_VERSION ?= latest

export TOOLS_BIN_DIR := $(TOOLS_BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

#########################################
# Tools                                 #
#########################################

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)

$(CONTROLLER_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

$(DEEPCOPY_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install k8s.io/code-generator/cmd/deepcopy-gen@$(CODE_GENERATOR_VERSION)

$(DEFAULTER_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install k8s.io/code-generator/cmd/defaulter-gen@$(CODE_GENERATOR_VERSION)

$(CONVERSION_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install k8s.io/code-generator/cmd/conversion-gen@$(CODE_GENERATOR_VERSION)

$(OPENAPI_GEN):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install k8s.io/kube-openapi/cmd/openapi-gen

$(VGOPATH):
	@if test -x $(TOOLS_BIN_DIR)/vgopath && ! $(TOOLS_BIN_DIR)/vgopath version | grep -q $(VGOPATH_VERSION); then \
		echo "$(TOOLS_BIN_DIR)/vgopath version is not expected $(VGOPATH_VERSION). Removing it before installing."; \
		rm -rf $(TOOLS_BIN_DIR)/vgopath; \
	fi
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/ironcore-dev/vgopath@$(VGOPATH_VERSION)

$(GEN_CRD_API_REFERENCE_DOCS):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/ahmetb/gen-crd-api-reference-docs@$(GEN_CRD_API_REFERENCE_DOCS_VERSION)

$(GOIMPORTS):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

$(GOLANGCI_LINT): $(TOOLS_BIN_DIR)
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)