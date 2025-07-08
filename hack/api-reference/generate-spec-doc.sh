# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

./hack/tools/bin/gen-crd-api-reference-docs -config "./hack/api-reference/providerspec-config.json" -api-dir "./pkg/apis/machine/v1alpha1" -template-dir "./hack/api-reference/template" -out-file="./docs/documents/apis.md"
sed 's/?id=//g' docs/documents/apis.md > docs/documents/apis-1.md
mv docs/documents/apis-1.md docs/documents/apis.md
