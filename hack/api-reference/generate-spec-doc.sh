# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

crd-ref-docs \
  --source-path="./pkg/apis/machine/v1alpha1" \
  --config="./hack/api-reference/config.yaml" \
  --renderer=markdown \
  --output-path="./docs/documents/apis.md"
