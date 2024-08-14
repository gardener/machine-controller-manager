#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o pipefail

##############################################################################################################
# Script sets up local development environment enabling you to start MCM process locally.
# It does the following:
# 1. Downloads short-lived control and target cluster kube-configs
# 2. Ensures that kube-configs are copied to both mcm and provider-mcm project directory
# 3. Scales down MCM to 0
# 4. Places annotation dependency-watchdog.gardener.cloud/ignore-scaling on MCM deployment, thus preventing
#    DWD from scaling it back up.
# 5. Updates Makefile for mcm and provider-mcm projects and exports variables used by the makefile.
##############################################################################################################

declare SEED SHOOT PROJECT PROVIDER # these are mandatory cli flags to be provided by the user
# these are optional cli flags
declare PROVIDER_MCM_PROJECT_DIR
declare RELATIVE_KUBECONFIG_PATH="dev/kube-configs"
declare LANDSCAPE_NAME="dev"
declare KUBECONFIG_EXPIRY_SECONDS="3600"
declare VIRTUAL_GARDEN_CLUSTER
declare ABSOLUTE_KUBE_CONFIG_PATH
declare ABSOLUTE_PROVIDER_KUBECONFIG_PATH

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"
PROJECT_DIR="$(cd "$(dirname "${SCRIPT_DIR}")" &>/dev/null && pwd)"

function create_usage() {
  usage=$(printf '%s\n' "
    Usage: $(basename $0) [Options]
    Options:
      -d | --seed                       <seed-cluster-name>                 (Required) Name of the Gardener Seed Cluster
      -t | --shoot                      <shoot-cluster-name>                (Required) Name of the Gardener Shoot Cluster
      -p | --project                    <project-name>                      (Required) Name of the Gardener Project
      -i | --provider                   <provider-name>                     (Required) Infrastructure provider name. Supported providers (gcp|aws|azure|vsphere|openstack|alicloud|metal|equinix-metal)
      -l | --landscape                  <landscape-name>                    (Optional) Name of the landscape. Defaults to dev
      -k | --kubeconfig-path            <relative-kubeconfig-path>          (Optional) Relative path to the <PROJECT-DIR> where kubeconfigs will be downloaded. Path should not start with '/'. Default: <PROJECT-DIR>/dev/kube-configs
      -m | --mcm-provider-project-path  <absolute-mcm-provider-project-dir> (Optional) MCM Provider project directory. If not provided then it assumes that both mcm and mcm-provider projects are under the same parent directory
      -e | --kubeconfig-expiry-seconds  <expiry-duration-in-seconds>        (Optional) Common expiry durations in seconds for control and target KubeConfigs. Default: 3600 seconds. Max accepted value is 86400 seconds.
    ")
  echo "${usage}"
}

function parse_flags() {
  while test $# -gt 0; do
    case "$1" in
    --seed | -d)
      shift
      SEED="$1"
      ;;
    --shoot | -t)
      shift
      SHOOT="$1"
      ;;
    --project | -p)
      shift
      PROJECT="$1"
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
    --kubeconfig-expiry-seconds | -e)
      shift
      KUBECONFIG_EXPIRY_SECONDS="$1"
      ;;
    --landscape | -l)
      shift
      LANDSCAPE_NAME="$1"
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
  if [[ -z "${SEED}" ]]; then
    echo -e "Seed has not been passed. Please provide Seed either by specifying --seed or -d argument"
    exit 1
  fi
  if [[ -z "${SHOOT}" ]]; then
    echo -e "Shoot has not been passed. Please provide Shoot either by specifying --shoot or -t argument"
    exit 1
  fi
  if [[ -z "${PROJECT}" ]]; then
    echo -e "Project name has not been passed. Please provide Project name either by specifying --project or -p argument"
    exit 1
  fi
  if [[ -z "${PROVIDER}" ]]; then
    echo -e "Infrastructure provider name has not been passed. Please provide infrastructure provider name either by specifying --provider or -i argument"
    exit 1
  fi
  if [[ -n "${KUBECONFIG_EXPIRY_SECONDS}" ]] && [[ "${KUBECONFIG_EXPIRY_SECONDS}" -gt 86400 ]]; then
    echo -e "KubeConfig expiration seconds cannot be greater than 86400"
    exit 1
  fi
}

function initialize() {
  if [[ -z "${PROVIDER_MCM_PROJECT_DIR}" ]]; then
    PROVIDER_MCM_PROJECT_DIR=$(dirname "${PROJECT_DIR}")/machine-controller-manager-provider-"${PROVIDER}"
  fi
  echo "Creating directory ${RELATIVE_KUBECONFIG_PATH} if it does not exist..."
  mkdir -p "${RELATIVE_KUBECONFIG_PATH}"
  ABSOLUTE_KUBE_CONFIG_PATH="$(
    cd "$(dirname "${RELATIVE_KUBECONFIG_PATH}")"
    pwd
  )/$(basename "${RELATIVE_KUBECONFIG_PATH}")"
  ABSOLUTE_PROVIDER_KUBECONFIG_PATH="${PROVIDER_MCM_PROJECT_DIR}/dev/kube-configs"
  echo "Creating directory ${ABSOLUTE_PROVIDER_KUBECONFIG_PATH} if it does not exist..."
  mkdir -p "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
  VIRTUAL_GARDEN_CLUSTER="sap-landscape-${LANDSCAPE_NAME}"
}

function download_kubeconfigs() {
  readonly garden_namespace="garden"
  local project_namespace="${garden_namespace}-${PROJECT}"

  kubeconfig_request_path=$(create_kubeconfig_request_yaml)

  echo "Targeting Virtual Garden Cluster ${VIRTUAL_GARDEN_CLUSTER}..."
  gardenctl target --garden "${VIRTUAL_GARDEN_CLUSTER}"
  eval "$(gardenctl kubectl-env zsh)"

  echo "Downloading kubeconfig for control cluster to ${ABSOLUTE_KUBE_CONFIG_PATH}..."
  kubectl create \
    -f "${kubeconfig_request_path}" \
    --raw "/apis/core.gardener.cloud/v1beta1/namespaces/${garden_namespace}/shoots/${SEED}/adminkubeconfig" |
    jq -r '.status.kubeconfig' |
    base64 -d >"${ABSOLUTE_KUBE_CONFIG_PATH}/kubeconfig_control.yaml"

  echo "Downloading kubeconfig for target cluster to ${ABSOLUTE_KUBE_CONFIG_PATH}..."
  kubectl create \
    -f "${kubeconfig_request_path}" \
    --raw "/apis/core.gardener.cloud/v1beta1/namespaces/${project_namespace}/shoots/${SHOOT}/adminkubeconfig" |
    jq -r '.status.kubeconfig' |
    base64 -d >"${ABSOLUTE_KUBE_CONFIG_PATH}/kubeconfig_target.yaml"

  echo "Removing generated admin kube config json..."
  rm -f "${SCRIPT_DIR}"/admin-kube-config-request.json
}

function create_kubeconfig_request_yaml() {
  local expiry_seconds template_path target_request_path
  expiry_seconds=${KUBECONFIG_EXPIRY_SECONDS}
  template_path="${SCRIPT_DIR}"/admin-kube-config-request-template.json
  target_request_path="${SCRIPT_DIR}"/admin-kube-config-request.json
  export expiry_seconds
  envsubst <"${template_path}" >"${target_request_path}"
  unset expiry_seconds
  echo "${target_request_path}"
}

function copy_kubeconfigs_to_provider_mcm() {
  for kc in "${ABSOLUTE_KUBE_CONFIG_PATH}"/*.yaml; do
    cp -v "${kc}" "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
  done
}

function scale_down_mcm() {
  gardenctl target --garden "${VIRTUAL_GARDEN_CLUSTER}" --project "${PROJECT}" --shoot "${SHOOT}" --control-plane
  eval "$(gardenctl kubectl-env zsh)"

  echo "annotating deployment/machine-controller-manager with dependency-watchdog.gardener.cloud/ignore-scaling=true..."
  kubectl annotate --overwrite=true deployment/machine-controller-manager dependency-watchdog.gardener.cloud/ignore-scaling=true
  # NOTE: One must remove the annotation after local setup is no longer needed. Use the below command to remove the annotation on machine-controller-manager deployment resource:
  # kubectl annotate --overwrite=true deployment/machine-controller-manager dependency-watchdog.gardener.cloud/ignore-scaling-
  echo "scaling down deployment/machine-controller-manager to 0..."
  kubectl scale deployment/machine-controller-manager --replicas=0
}

# create_makefile_env creates a .env file that will get copied to both mcm and mcm-provider project directories for their
# respective Makefile to use.
function set_makefile_env() {
  if [[ $# -ne 2 ]]; then
    echo -e "${FUNCNAME[0]} expects two arguments - project-directory and target-kube-config-path"
  fi
  local target_project_dir target_kube_config_path
  target_project_dir="$1"
  target_kube_config_path="$2"
  echo "IS_CONTROL_CLUSTER_SEED=true" > "${target_project_dir}/.env"
  {
    printf "CONTROL_CLUSTER_NAMESPACE=shoot--%s--%s\n" "${PROJECT}" "${SHOOT}";
    printf "CONTROL_NAMESPACE=%s--%s\n" "${PROJECT}" "${SHOOT}";
    printf "CONTROL_KUBECONFIG=%s\n" "${target_kube_config_path}/kubeconfig_control.yaml";
    printf "TARGET_KUBECONFIG=%s\n" "${target_kube_config_path}/kubeconfig_target.yaml";
    printf "LEADER_ELECT=false\n"
  } >>"${target_project_dir}/.env"
}

function main() {
  parse_flags "$@"
  validate_args
  initialize
  download_kubeconfigs
  copy_kubeconfigs_to_provider_mcm
  scale_down_mcm
  set_makefile_env "${PROJECT_DIR}" "${ABSOLUTE_KUBE_CONFIG_PATH}"
  set_makefile_env "${PROVIDER_MCM_PROJECT_DIR}" "${ABSOLUTE_PROVIDER_KUBECONFIG_PATH}"
}

USAGE=$(create_usage)
main "$@"
