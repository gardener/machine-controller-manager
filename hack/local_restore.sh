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
# Script restores MCM deployment and removes the annotation dependency-watchdog.gardener.cloud/ignore-scaling
# This script should be called once you are done using local_setup_for_gardener.sh
##############################################################################################################

declare SHOOT PROJECT PROVIDER
declare PROVIDER_MCM_PROJECT_DIR
declare RELATIVE_KUBECONFIG_PATH="dev/kube-configs"
declare LANDSCAPE_NAME="dev"


SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd "$(dirname "${SCRIPT_DIR}")" &>/dev/null && pwd)"

function create_usage() {
  usage=$(printf '%s\n' "
    Usage: $(basename $0) [Options]
    Options:
      -t | --shoot                      <shoot-cluster-name>                (Required) Name of the Gardener Shoot Cluster
      -p | --project                    <project-name>                      (Required) Name of the Gardener Project
      -l | --landscape                  <landscape-name>                    (Optional) Name of the landscape. Defaults to dev
      -i | --provider                   <provider-name>                     (Required) Infrastructure provider name. Supported providers (gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)
      -k | --kubeconfig-path            <relative-kubeconfig-path>          (Optional) Relative path to the <PROJECT-DIR> where kubeconfigs will be downloaded. Path should not start with '/'. Default: <PROJECT-DIR>/dev/kube-configs
      -m | --mcm-provider-project-path  <absolute-mcm-provider-project-dir> (Optional) MCM Provider project directory. If not provided then it assumes that both mcm and mcm-provider projects are under the same parent directory
    ")
  echo "${usage}"
}

function parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --shoot | -t)
      shift
      SHOOT="$1"
      ;;
    --project | -p)
      shift
      PROJECT="$1"
      ;;
    --landscape | -l)
      shift
      LANDSCAPE_NAME="$1"
      ;;
    --provider | -i)
      shift
      PROVIDER="$1"
      ;;
    --kubeconfig-path | -k)
      shift
      RELATIVE_KUBECONFIG_PATH="$1"
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
  if [[ -z "${SHOOT}" ]]; then
    echo -e "Shoot has not been passed. Please provide Shoot either by specifying --shoot or -t argument"
    exit 1
  fi
  if [[ -z "${PROJECT}" ]]; then
    echo -e "Project name has not been passed. Please provide Project name either by specifying --project or -p argument"
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
  local virtual_garden_cluster="sap-landscape-${LANDSCAPE_NAME}"
  gardenctl target --garden "${virtual_garden_cluster}" --project "${PROJECT}" --shoot "${SHOOT}" --control-plane
  eval "$(gardenctl kubectl-env zsh)"
  echo "removing annotation dependency-watchdog.gardener.cloud/ignore-scaling from mcm deployment..."
  kubectl annotate --overwrite=true deployment/machine-controller-manager dependency-watchdog.gardener.cloud/ignore-scaling-
  echo "scaling up mcm deployment to 1..."
  kubectl scale deployment/machine-controller-manager --replicas=1
}

function delete_generated_configs() {
  echo "Removing downloaded kubeconfigs..."
  rm -f "${ABSOLUTE_KUBE_CONFIG_PATH}"/*.yaml
  rm -f "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"/*.yaml
  echo "Clearing .env files..."
  sed -r -i '/^#/!d' "${PROJECT_DIR}"/.env
  sed -r -i '/^#/!d' "${PROVIDER_MCM_PROJECT_DIR}"/.env
  echo "Removing generated admin kube config json..."
  rm -f "${SCRIPT_DIR}"/admin-kube-config-request.json
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