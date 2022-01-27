cd ./hack/api-reference
./gen-crd-api-reference-docs -config "providerspec-config.json" -api-dir "../../pkg/apis/machine/v1alpha1" -out-file="../../docs/docs/apis.md"
sed 's/?id=//g' ../../docs/docs/apis.md > ../../docs/docs/apis-1.md
mv ../../docs/docs/apis-1.md ../../docs/docs/apis.md