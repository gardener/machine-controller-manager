#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
export TERM="xterm-256color"

bold="$(tput bold)"
blue="$(tput setaf 4)"
normal="$(tput sgr0)"

function qualify-gvs() {
  APIS_PKG="$1"
  GROUPS_WITH_VERSIONS="$2"
  join_char=""
  res=""

  for GVs in ${GROUPS_WITH_VERSIONS}; do
    IFS=: read -r G Vs <<<"${GVs}"

    for V in ${Vs//,/ }; do
      res="$res$join_char$APIS_PKG/$G/$V"
      join_char=","
    done
  done

  echo "$res"
}

function qualify-gs() {
  APIS_PKG="$1"
  unset GROUPS
  IFS=' ' read -ra GROUPS <<< "$2"
  join_char=""
  res=""

  for G in "${GROUPS[@]}"; do
    res="$res$join_char$APIS_PKG/$G"
    join_char=","
  done

  echo "$res"
}

VGOPATH="$VGOPATH"
DEEPCOPY_GEN="$DEEPCOPY_GEN"
DEFAULTER_GEN="$DEFAULTER_GEN"
CONVERSION_GEN="$CONVERSION_GEN"

VIRTUAL_GOPATH="$(mktemp -d)"
trap 'rm -rf "$GOPATH"' EXIT

# Setup virtual GOPATH so the codegen tools work as expected.
(cd "$SCRIPT_DIR/.."; go mod download && "$VGOPATH" -o "$VIRTUAL_GOPATH")

export GOROOT="${GOROOT:-"$(go env GOROOT)"}"
export GOPATH="$VIRTUAL_GOPATH"
export GO111MODULE=off

echo "${bold}Public types${normal}"

echo "Generating ${blue}deepcopy${normal}"
"$DEEPCOPY_GEN" \
  --output-base "$GOPATH/src" \
  --go-header-file "$SCRIPT_DIR/LICENSE_BOILERPLATE.txt" \
  --input-dirs "$(qualify-gvs "github.com/gardener/machine-controller-manager/pkg/apis" "machine:v1alpha1")" \
  -O zz_generated.deepcopy

echo "${bold}Internal types${normal}"

echo "Generating ${blue}deepcopy${normal}"
"$DEEPCOPY_GEN" \
  --output-base "$GOPATH/src" \
  --go-header-file "$SCRIPT_DIR/LICENSE_BOILERPLATE.txt" \
  --input-dirs "$(qualify-gs "github.com/gardener/machine-controller-manager/pkg/apis" "machine")" \
  -O zz_generated.deepcopy

echo "Generating ${blue}defaulter${normal}"
"$DEFAULTER_GEN" \
  --output-base "$GOPATH/src" \
  --go-header-file "$SCRIPT_DIR/LICENSE_BOILERPLATE.txt" \
  --input-dirs "$(qualify-gvs "github.com/gardener/machine-controller-manager/pkg/apis" "machine:v1alpha1")" \
  -O zz_generated.defaults

echo "Generating ${blue}conversion${normal}"
"$CONVERSION_GEN" \
  --output-base "$GOPATH/src" \
  --go-header-file "$SCRIPT_DIR/LICENSE_BOILERPLATE.txt" \
  --input-dirs "$(qualify-gs "github.com/gardener/machine-controller-manager/pkg/apis" "machine")" \
  --input-dirs "$(qualify-gvs "github.com/gardener/machine-controller-manager/pkg/apis" "machine:v1alpha1")" \
  -O zz_generated.conversion
