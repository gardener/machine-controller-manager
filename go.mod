module github.com/gardener/machine-controller-manager

go 1.16

require (
	github.com/Azure/azure-sdk-for-go v42.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.11.1
	github.com/Azure/go-autorest/autorest/adal v0.9.5
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/aliyun/alibaba-cloud-sdk-go v0.0.0-20180828111155-cad214d7d71f
	github.com/aws/aws-sdk-go v1.23.13
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/gophercloud/gophercloud v0.7.0
	github.com/gophercloud/utils v0.0.0-20200204043447-9864b6f1f12f
	github.com/onsi/ginkgo v1.16.2
	github.com/onsi/gomega v1.11.0
	github.com/packethost/packngo v0.0.0-20181217122008-b3b45f1b4979
	github.com/prometheus/client_golang v1.7.1
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/spf13/pflag v1.0.5
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	google.golang.org/api v0.20.0
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/apiserver v0.0.0-20190918160949-bfa5e2e684ad
	k8s.io/client-go v0.20.5
	k8s.io/cluster-bootstrap v0.0.0-20190918163108-da9fdfce26bb
	k8s.io/code-generator v0.0.0-20190912054826-cd179ad6a269
	k8s.io/component-base v0.20.5
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

replace (
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.5.0
	k8s.io/api => k8s.io/api v0.20.5
	k8s.io/apimachinery => k8s.io/apimachinery v0.20.6-rc.0
	k8s.io/apiserver => k8s.io/apiserver v0.20.5
	k8s.io/client-go => k8s.io/client-go v0.20.5
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.5
	k8s.io/code-generator => k8s.io/code-generator v0.20.6-rc.0
	k8s.io/kube-openapi => github.com/gardener/kube-openapi v0.0.0-20201221124747-75e88872edcf // k8s-1.19
)
