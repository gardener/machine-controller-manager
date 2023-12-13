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

cd ./hack/api-reference
GO111MODULE=on go install github.com/ahmetb/gen-crd-api-reference-docs@latest
"$GOBIN"/gen-crd-api-reference-docs -config "providerspec-config.json" -api-dir "../../pkg/apis/machine/v1alpha1" -out-file="../../docs/documents/apis.md"sed 's/?id=//g' ../../docs/documents/apis.md > ../../docs/documents/apis-1.md
mv ../../docs/documents/apis-1.md ../../docs/documents/apis.md