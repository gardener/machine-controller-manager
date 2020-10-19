module github.com/gardener/machine-controller-manager

go 1.14

require (
	github.com/Azure/azure-sdk-for-go v42.2.0+incompatible
	github.com/Azure/go-autorest/autorest v0.10.1
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/Azure/go-autorest/autorest/to v0.3.0
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/aliyun/alibaba-cloud-sdk-go v0.0.0-20180828111155-cad214d7d71f
	github.com/aws/aws-sdk-go v1.13.54
	github.com/davecgh/go-spew v1.1.1
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-ini/ini v1.36.0 // indirect
	github.com/go-openapi/spec v0.19.3
	github.com/golang/groupcache v0.0.0-20180513044358-24b0969c4cb7 // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gophercloud/gophercloud v0.7.0
	github.com/gophercloud/utils v0.0.0-20200204043447-9864b6f1f12f
	github.com/jmespath/go-jmespath v0.0.0-20160202185014-0b12d6b521d8 // indirect
	github.com/json-iterator/go v1.1.9 // indirect
	github.com/onsi/ginkgo v1.11.0
	github.com/onsi/gomega v1.7.0
	github.com/packethost/packngo v0.0.0-20181217122008-b3b45f1b4979
	github.com/prometheus/client_golang v1.3.0
	github.com/satori/go.uuid v1.2.0 // indirect
	github.com/smartystreets/goconvey v0.0.0-20190330032615-68dc04aab96a // indirect
	github.com/spf13/pflag v1.0.5
	golang.org/x/lint v0.0.0-20190313153728-d0100b6bd8b3
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	google.golang.org/api v0.4.0
	gopkg.in/ini.v1 v1.42.0 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/apiserver v0.18.8
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/cluster-bootstrap v0.18.8
	k8s.io/code-generator v0.18.8
	k8s.io/component-base v0.18.8
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29 // release-1.18
	k8s.io/utils v0.0.0-20200603063816-c1c6865ac451
)

replace (
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega => github.com/onsi/gomega v1.5.0
	k8s.io/api => k8s.io/api v0.18.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.8
	k8s.io/apiserver => k8s.io/apiserver v0.18.8
	k8s.io/client-go => k8s.io/client-go v0.18.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.8
	k8s.io/code-generator => k8s.io/code-generator v0.18.8
	k8s.io/kube-openapi => github.com/gardener/kube-openapi v0.0.0-20200831104310-b5db060350c7
)
