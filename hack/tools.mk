# Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

TOOLS_DIR                  := hack/tools
TOOLS_BIN_DIR              := $(TOOLS_DIR)/bin
GO_ADD_LICENSE             := $(TOOLS_BIN_DIR)/addlicense

# default tool versions
GO_ADD_LICENSE_VERSION ?= latest

export TOOLS_BIN_DIR := $(TOOLS_BIN_DIR)
export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
$(info "TOOLS_BIN_DIR from tools.mk", $(TOOLS_BIN_DIR))
$(info "TOOLS_DIR from tools.mk", $(TOOLS_DIR))
$(info "PATH from tools.mk", $(PATH))

#########################################
# Tools                                 #
#########################################

$(GO_ADD_LICENSE):
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install github.com/google/addlicense@$(GO_ADD_LICENSE_VERSION)