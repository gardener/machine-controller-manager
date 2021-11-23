module github.com/gardener/machine-controller-manager

go 1.16

require (
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/davecgh/go-spew v1.1.1
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-openapi/spec v0.19.3
	github.com/onsi/ginkgo v1.16.2
	github.com/onsi/gomega v1.11.0
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/pflag v1.0.5
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b
	golang.org/x/sys v0.0.0-20210514084401-e8d321eab015 // indirect
	k8s.io/api v0.20.6
	k8s.io/apiextensions-apiserver v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/apiserver v0.20.6
	k8s.io/client-go v0.20.6
	k8s.io/cluster-bootstrap v0.0.0-20190918163108-da9fdfce26bb
	k8s.io/code-generator v0.20.6
	k8s.io/component-base v0.20.6
	k8s.io/klog/v2 v2.4.0
	k8s.io/kube-openapi v0.0.0-20201113171705-d219536bb9fd // keep this value in sync with k8s.io/apiserver
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)
