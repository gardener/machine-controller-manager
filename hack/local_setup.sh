# /*
# Copyright (c) YEAR SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#      http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
# */


# THE SCRIPT SHOULD BE RUN WHILE IN THE `machine-controller-manager/hack` FOLDER & IT ASSUMES THAT IN THE PARENT DIRECTORY OF MCM THE MCM PROVIDER IS ALSO PRESENT
# THE SCRIPT ALSO ASSUMES THAT THERE THE `machine-controller-manager-provider-<provider-name>/dev/kubeconfigs` FOLDER EXISTS.

#HOW TO CALL 
################################################################################################

# ./local_setup.sh --PROJECT <project-name> --SEED <seed-name> --SHOOT <shoot-name> --PROVIDER <cluster-provider-name>

################################################################################################
#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

declare SEED 
declare SHOOT
declare PROJECT 
declare PROVIDER
declare CURRENT_DIR
declare PROJECT_ROOT
declare KUBECONFIG_PATH
declare PROVIDER_PATH
declare PROVIDER_KUBECONFIG_PATH

while [ $# -gt 0 ]; do

if [[ $1 == *"--"* ]]; then
     v="${1/--/}"
     declare $v="$2"
fi

shift
done

main() {
     setPaths
     echo "Targeting gardener to get the kubeconfig of ${SHOOT} cluster control plane"
     setControlKubeconfig
     echo "Targeting seed cluster to get the kubeconfig of ${SHOOT} cluster"
     setTargetKubeconfig
     echo "Placing kubeconfigs at /dev/kubeconfigs/kubeconfig_<target/control>.yaml"
     copyFilesToProvider
     # All the kubeconfigs are at place
     echo "Annotating deployment/machine-controller-manager with dependency-watchdog.gardener.cloud/ignore-scaling=true"
     addAnnotation
     echo "Scaling down deployment/machine-controller-manager to 0"
     scaleDownMCM
     echo "Updating and exporting variables for starting the local MCM instance"
     updateMCMMakefile
     exportVariables
     updateProviderMakefile
}

setPaths() {
     CURRENT_DIR=$(dirname $0)
     PROJECT_ROOT="${CURRENT_DIR}"/..
     KUBECONFIG_PATH="${PROJECT_ROOT}"/dev/kubeconfigs
     PROVIDER_PATH="${PROJECT_ROOT}"/../machine-controller-manager-provider-${PROVIDER}
     PROVIDER_KUBECONFIG_PATH="${PROVIDER_PATH}"/dev/kubeconfigs
}

setControlKubeconfig() {
     gardenctl target --garden sap-landscape-dev --project garden

     eval $(gardenctl kubectl-env bash)

     echo "$(kubectl get secret/"${SEED}".kubeconfig --template={{.data.kubeconfig}} | base64 -d)" > "${KUBECONFIG_PATH}"/kubeconfig_control.yaml
}

setTargetKubeconfig() {

     #setting up the target config

     gardenctl target --garden sap-landscape-dev --project "${PROJECT}" --shoot "${SHOOT}" --control-plane  

     eval $(gardenctl kubectl-env bash)

     secret=$(kubectl get secrets | awk '{ print $1 }' |grep user-kubeconfig-)

     echo "$(kubectl get secret/"${secret}" -n shoot--"${PROJECT}"--"${SHOOT}" --template={{.data.kubeconfig}} | base64 -d)" > "${KUBECONFIG_PATH}"/kubeconfig_target.yaml
}

copyFilesToProvider() {
     # Copy both the kubeconfigs to dev folder of provider 

     cp "${KUBECONFIG_PATH}"/kubeconfig_target.yaml "${PROVIDER_KUBECONFIG_PATH}"/kubeconfig_target.yaml
     cp "${KUBECONFIG_PATH}"/kubeconfig_control.yaml "${PROVIDER_KUBECONFIG_PATH}"/kubeconfig_control.yaml
}

addAnnotation() {
     # Adding annotation

     kubectl annotate --overwrite=true deployment/machine-controller-manager dependency-watchdog.gardener.cloud/ignore-scaling=true

     # Another way of adding annotation
     # kubectl patch deployment/machine-controller-manager -p '{"metadata":{"annotations":{"dependency-watchdog.gardener.cloud/ignore-scaling":true}} }'

     # Removing the annotation, needs to be called after done with local setup

     # kubectl annotate --overwrite=true deployment/machine-controller-manager dependency-watchdog.gardener.cloud/ignore-scaling-

}

scaleDownMCM() {
     # Scale down the MCM
     kubectl scale deployment/machine-controller-manager --replicas=0

     # If delete is required, when Scale Down operation is not working due to some reason
     # kubectl delete deployment/machine-controller-manager
}

updateMCMMakefile() {
     # Point the makefiles to the correct kubeconfigs to use for local run
     sed -i -e "s/\(CONTROL_NAMESPACE *:=\).*/\1 shoot--"${PROJECT}"--"${SHOOT}"/1" "${PROJECT_ROOT}"/makefile
     sed -i -e "s/\(CONTROL_KUBECONFIG *:=\).*/\1 dev\/kubeconfigs\/kubeconfig_control.yaml/1" "${PROJECT_ROOT}"/makefile
     sed -i -e "s/\(TARGET_KUBECONFIG *:=\).*/\1 dev\/kubeconfigs\/kubeconfig_target.yaml/1" "${PROJECT_ROOT}"/makefile
}

exportVariables() {
     export CONTROL_NAMESPACE=shoot--"${PROJECT}"--"${SHOOT}"
     export CONTROL_KUBECONFIG="${KUBECONFIG_PATH}"/kubeconfig_control.yaml
     export TARGET_KUBECONFIG="${KUBECONFIG_PATH}"/kubeconfig_target.yaml
}

updateProviderMakefile() {
     # Makefile of provider also needs to point at those kubeconfigs
     sed -i -e "s/\(CONTROL_NAMESPACE *:= *\).*/\1 shoot--"${PROJECT}"--"${SHOOT}"/1" "${PROVIDER_PATH}"/makefile
     sed -i -e "s/\(CONTROL_KUBECONFIG *:= *\).*/\1 dev\/kubeconfigs\/kubeconfig_control.yaml/1" "${PROVIDER_PATH}"/makefile
     sed -i -e "s/\(TARGET_KUBECONFIG *:= *\).*/\1 dev\/kubeconfigs\/kubeconfig_target.yaml/1" "${PROVIDER_PATH}"/makefile
}

main
