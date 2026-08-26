# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

cd ./hack/api-reference
../tools/bin/gen-crd-api-reference-docs -config "providerspec-config.json" -api-dir "../../pkg/apis/machine/v1alpha1" -out-file="../../docs/documents/apis.md"
sed 's/?id=//g' ../../docs/documents/apis.md > ../../docs/documents/apis-1.md
mv ../../docs/documents/apis-1.md ../../docs/documents/apis.md