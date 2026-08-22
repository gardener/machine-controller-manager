# SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

PKG_ROOT      := $(shell dirname "$(realpath $(lastword $(MAKEFILE_LIST)))")
PKG_DIR       := $(shell dirname "$(PKG_ROOT)")
REPO_ROOT     := $(shell dirname "$(PKG_DIR)")
TOOLS_BIN_DIR := $(REPO_ROOT)/hack/tools/bin

KWOKCTL       ?= $(TOOLS_BIN_DIR)/kwokctl
KWOK_VERSION  ?= v0.8.0

$(KWOKCTL): $(TOOLS_BIN_DIR)
	GOBIN=$(abspath $(TOOLS_BIN_DIR)) go install sigs.k8s.io/kwok/cmd/{kwok,kwokctl}@$(KWOK_VERSION)


.PHONY: build
build:
	@mkdir -p bin
	go build -v -o bin/machine-controller cmd/machine-controller/main.go

.PHONY: start
start:
	@GO111MODULE=on go run \
			cmd/machine-controller/main.go \
			--control-kubeconfig=$(CONTROL_KUBECONFIG) \
			--target-kubeconfig=$(TARGET_KUBECONFIG) \
			--namespace=$(CONTROL_NAMESPACE) \
			--machine-creation-timeout=20m \
			--machine-drain-timeout=5m \
			--machine-health-timeout=10m \
			--machine-pv-detach-timeout=2m \
			--machine-safety-apiserver-statuscheck-timeout=30s \
			--machine-safety-apiserver-statuscheck-period=1m \
			--machine-safety-orphan-vms-period=30m \
			--leader-elect=$(LEADER_ELECT) \
			--v=3

.PHONY: start-simulated-cluster
start-simulated-cluster: $(KWOKCTL)
	$(KWOKCTL) create cluster --name "test-cluster" --config ./cluster/kwok-config.yaml

.PHONY: stop-simulated-cluster
stop-simulated-cluster: $(KWOKCTL)
	$(KWOKCTL) delete cluster --name "test-cluster"

.PHONY: test-integration
test-integration:
	@(cd test/integration/controller; ginkgo -v --show-node-events --poll-progress-after=300s --poll-progress-interval=60s)
