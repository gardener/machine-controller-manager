# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

TOOLS_DIR                  := hack/tools
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin
GO_ADD_LICENSE             := $(TOOLS_BIN_DIR)/addlicense

# default tool versions
GO_ADD_LICENSE_VERSION ?= latest

export TOOLS_BIN_DIR := $(TOOLS_BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

#########################################
# Tools                                 #
#########################################

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)