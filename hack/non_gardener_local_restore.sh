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
# Script restores MCM deployment and cleans up any temporary files that were required or created by its
# counter-part script  non_gardener_local_setup.sh.
# This script should be called once you are done using gardener_local_setup.sh
##############################################################################################################

declare PROVIDER NAMESPACE CONTROL_KUBECONFIG_PATH
declare PROVIDER_MCM_PROJECT_DIR
declare RELATIVE_KUBECONFIG_PATH="dev/kube-configs"


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd "$(dirname "${SCRIPT_DIR}")" &>/dev/null && pwd)"

function create_usage() {
  usage=$(printf '%s\n' "
    Usage: $(basename $0) [Options]
    Options:
      -n | --namespace                  <namespace>                         (Required) This is the namespace where MCM pods are deployed.
      -c | --control-kubeconfig-path    <control-kubeconfig-path>           (Required) Kubeconfig file path which points to the control-plane of cluster where MCM is running.
      -i | --provider                   <provider-name>                     (Required) Infrastructure provider name. Supported providers (gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)
      -m | --mcm-provider-project-path  <absolute-mcm-provider-project-dir> (Optional) MCM Provider project directory. If not provided then it assumes that both mcm and mcm-provider projects are under the same parent directory
    ")
  echo "${usage}"
}

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
    --provider | -i)
      shift
      PROVIDER="$1"
      ;;
    --mcm-provider-project-path | -m)
      shift
      PROVIDER_MCM_PROJECT_DIR="$1"
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

function validate_args() {
  if [[ -z "${NAMESPACE}" ]]; then
    echo -e "Namespace has not been passed. Please provide it either by specifying --namespace or -n argument"
    exit 1
  fi
  if [[ -z "${CONTROL_KUBECONFIG_PATH}" ]]; then
    echo -e "Kubeconfig file path for control cluster has not been passed. Please provide it either by specifying --control-kubeconfig-path or -c argument"
    exit 1
  fi
  if [[ -z "${PROVIDER}" ]]; then
    echo -e "Provider name has not been passed. Please provide it either by specifying --provider or -p argument"
    exit 1
  fi
}

function initialize_variables() {
  if [[ -z "${PROVIDER_MCM_PROJECT_DIR}" ]]; then
    PROVIDER_MCM_PROJECT_DIR=$(dirname "${PROJECT_DIR}")/machine-controller-manager-provider-"${PROVIDER}"
  fi
  ABSOLUTE_KUBE_CONFIG_PATH="$(cd "$(dirname "${RELATIVE_KUBECONFIG_PATH}")"; pwd)/$(basename "${RELATIVE_KUBECONFIG_PATH}")"
  ABSOLUTE_PROVIDER_KUBECONFIG_PATH="${PROVIDER_MCM_PROJECT_DIR}/dev/kube-configs"
}

function restore_mcm_deployment() {
  echo "scaling up mcm deployment to 1..."
  KUBECONFIG="${CONTROL_KUBECONFIG_PATH}" kubectl scale deployment/machine-controller-manager --replicas=1
}

function delete_generated_configs() {
  echo "Removing downloaded kubeconfigs..."
  rm -f "${ABSOLUTE_KUBE_CONFIG_PATH}"/*.yaml
  rm -f "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"/*.yaml
  echo "Clearing .env files..."
  sed -r -i '/^#/!d' "${PROJECT_DIR}"/.env
  sed -r -i '/^#/!d' "${PROVIDER_MCM_PROJECT_DIR}"/.env
}

function main() {
  parse_flags "$@"
  validate_args
  initialize_variables
  restore_mcm_deployment
  delete_generated_configs
}

USAGE=$(create_usage)
main "$@"