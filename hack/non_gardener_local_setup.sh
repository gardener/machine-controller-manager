#!/usr/bin/env bash
# Copyright 2023 SAP SE or an SAP affiliate company
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

set -o errexit
set -o pipefail

##############################################################################################################
# Script sets up local development environment enabling you to start MCM process locally. This is a non-gardener
# setup. Therefore it is left to the consumer to ensure that user configured in the kubeconfig has sufficient
# permissions to modify the machine-controller-manager deployment.
#
# It does the following:
# 1. Ensures that kube-configs are copied to both mcm and provider-mcm project directory
# 2. Scales down MCM to 0
# 3. Updates Makefile for mcm and provider-mcm projects and exports variables used by the makefile.
##############################################################################################################

# these are mandatory cli flags to be provided by the user
declare PROVIDER NAMESPACE CONTROL_KUBECONFIG_PATH TARGET_KUBECONFIG_PATH

# these are optional cli flags
declare PROVIDER_MCM_PROJECT_DIR
declare ABSOLUTE_PROVIDER_KUBECONFIG_PATH

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd "$(dirname "${SCRIPT_DIR}")" &>/dev/null && pwd)"

# create_usage creates a CLI usage string.
function create_usage() {
  usage=$(printf '%s\n' "
    Usage: $(basename $0) [Options]
    Options:
      -n    | --namespace                  <namespace>                         (Required) This is the namespace where MCM pods are deployed.
      -c    | --control-kubeconfig-path    <control-kubeconfig-path>           (Required) Kubeconfig file path which points to the control-plane of cluster where MCM is running.
      -t    | --target-kubeconfig-path     <target-kubeconfig-path>            (Required) Kubeconfig file path which points to control plane of the cluster where nodes are created.
      -mcc  | --machineclass               <machineclass-v1>                   (Required) MachineClassV1 name. This is the machineclass that will be used to create the nodes.
      -i    | --provider                   <provider-name>                     (Required) Infrastructure provider name. Supported providers (gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)
      -m    | --mcm-provider-project-path  <absolute-mcm-provider-project-dir> (Optional) MCM Provider project directory. If not provided then it assumes that both mcm and mcm-provider projects are under the same parent directory
    ")
  echo "${usage}"
}

# parse_flags parses the CLI arguments passed by the consumer.
function parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --namespace | -n)
      shift
      NAMESPACE="$1"
      ;;
    --control-kubeconfig-path | -c)
      shift
      CONTROL_KUBECONFIG_PATH="$1"
      ;;
    --target-kubeconfig-path | -t)
      shift
      TARGET_KUBECONFIG_PATH="$1"
      ;;
    --provider | -i)
      shift
      PROVIDER="$1"
      ;;
    --mcm-provider-project-path | -m)
      shift
      PROVIDER_MCM_PROJECT_DIR="$1"
      ;;
    --machineclass | -mcc)
      shift
      MACHINECLASS="$1"
      ;;
    --help | -h)
      shift
      echo "${USAGE}"
      exit 0
      ;;
    esac
    shift
  done
}

# validate_args validates the CLI arguments being passed.
function validate_args() {
  if [[ -z "${NAMESPACE}" ]]; then
    echo -e "Namespace is not provided. Please provided it either by specifying --namespace or -n"
    exit 1
  fi
  if [[ -z "${CONTROL_KUBECONFIG_PATH}" ]]; then
    echo -e "Control Kubeconfig Path is not provided. Please provide it either by specifying --control-kubeconfig-path or -c argument"
    exit 1
  fi
  if [[ ! -f "${CONTROL_KUBECONFIG_PATH}" ]]; then
    echo -e "File ${CONTROL_KUBECONFIG_PATH} does not exist. Please ensure that the control kubeconfig is present."
    exit 1
  fi
  if [[ -z "${TARGET_KUBECONFIG_PATH}" ]]; then
    echo -e "Target Kubeconfig Path is not provided. Please provide it either by specifying --target-kubeconfig-path or -t argument"
    exit 1
  fi
  if [[ ! -f "${TARGET_KUBECONFIG_PATH}" ]]; then
    echo -e "File ${TARGET_KUBECONFIG_PATH} does not exist. Please ensure that the target kubeconfig is present."
    exit 1
  fi
  if [[ -z "${PROVIDER}" ]]; then
    echo -e "Infrastructure provider name has not been passed. Please provide infrastructure provider name either by specifying --provider or -i argument"
    exit 1
  fi
  if [[ -z "${MACHINECLASS}" ]]; then
    echo -e "MACHINECLASS has not been passed. Please provide MACHINECLASS either by specifying --machineclass or -mcc argument"
    exit 1
  fi
}

function init_provider_paths() {
  if [[ -z "${PROVIDER_MCM_PROJECT_DIR}" ]]; then
    PROVIDER_MCM_PROJECT_DIR=$(dirname "${PROJECT_DIR}")/machine-controller-manager-provider-"${PROVIDER}"
  fi
  ABSOLUTE_PROVIDER_KUBECONFIG_PATH="${PROVIDER_MCM_PROJECT_DIR}/dev/kube-configs"
  echo "Creating directory ${ABSOLUTE_PROVIDER_KUBECONFIG_PATH} if it does not exist..."
  mkdir -p "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
}

# copy_kubeconfigs_to_provider_mcm copies the downloaded or manually put kubeconfig files to the MCM provider git repo.
function copy_kubeconfigs_to_provider_mcm() {
  cp "${CONTROL_KUBECONFIG_PATH}" "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
  cp "${TARGET_KUBECONFIG_PATH}" "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
}

function scale_down_mcm() {
  echo "scaling down deployment/machine-controller-manager to 0..."
  set +e
  KUBECONFIG="${CONTROL_KUBECONFIG_PATH}" kubectl -n "${NAMESPACE}" scale deployment/machine-controller-manager --replicas=0
  if [[ $? -ne 0 ]]; then
    echo "MCM does not exist or failed to scale down deployment/machine-controller-manager to 0"
  fi
  set -e
}

# create_makefile_env creates a .env file that will get copied to both mcm and mcm-provider project directories for their
# respective Makefile to use.
function set_makefile_env() {
  if [[ $# -ne 1 ]]; then
    echo -e "${FUNCNAME[0]} expects one argument - project-directory"
  fi

  local target_project_dir="$1"
  {
    printf "\n%s" "IS_CONTROL_CLUSTER_SEED=false" > "${target_project_dir}/.env"
    printf "\n%s" "CONTROL_NAMESPACE=${NAMESPACE}" >> "${target_project_dir}/.env"
    printf "\n%s" "CONTROL_KUBECONFIG=${CONTROL_KUBECONFIG_PATH}" >>"${target_project_dir}/.env"
    printf "\n%s" "TARGET_KUBECONFIG=${TARGET_KUBECONFIG_PATH}" >>"${target_project_dir}/.env"
    printf "\n%s" "MACHINECLASS_V1=${MACHINECLASS}" >>"${target_project_dir}/.env"
  } >>"${target_project_dir}/.env"
}

# main is the entry point to this script.
function main() {
  parse_flags "$@"
  validate_args
  init_provider_paths
  copy_kubeconfigs_to_provider_mcm
  scale_down_mcm
  set_makefile_env "${PROJECT_DIR}"
  set_makefile_env "${PROVIDER_MCM_PROJECT_DIR}"
}

USAGE=$(create_usage)
main "$@"
