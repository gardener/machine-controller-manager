// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencorescheme "github.com/gardener/gardener/pkg/client/core/clientset/versioned/scheme"
	gardenextensionsscheme "github.com/gardener/gardener/pkg/client/extensions/clientset/versioned/scheme"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	resourcesscheme "github.com/gardener/gardener-resource-manager/pkg/apis/resources/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	autoscalingscheme "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	corescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiregistrationscheme "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// GardenScheme is the scheme used in the Garden cluster.
	GardenScheme = runtime.NewScheme()
	// SeedScheme is the scheme used in the Seed cluster.
	SeedScheme = runtime.NewScheme()
	// ShootScheme is the scheme used in the Shoot cluster.
	ShootScheme = runtime.NewScheme()
	// PlantScheme is the scheme used in the Plant cluster
	PlantScheme = runtime.NewScheme()

	// DefaultDeleteOptions use foreground propagation policy and grace period of 60 seconds.
	DefaultDeleteOptions = []client.DeleteOption{
		client.PropagationPolicy(metav1.DeletePropagationForeground),
		client.GracePeriodSeconds(60),
	}
	// ForceDeleteOptions use background propagation policy and grace period of 0 seconds.
	ForceDeleteOptions = []client.DeleteOption{
		client.PropagationPolicy(metav1.DeletePropagationBackground),
		client.GracePeriodSeconds(0),
	}

	// ShootSerializer is a YAML serializer using the Shoot scheme.
	ShootSerializer = json.NewSerializerWithOptions(json.DefaultMetaFactory, ShootScheme, ShootScheme, json.SerializerOptions{Yaml: true, Pretty: false, Strict: false})
	// ShootCodec is a codec factory using the Shoot scheme.
	ShootCodec = serializer.NewCodecFactory(ShootScheme)
)

// DefaultGetOptions are the default options for GET requests.
func DefaultGetOptions() metav1.GetOptions { return metav1.GetOptions{} }

// DefaultCreateOptions are the default options for CREATE requests.
func DefaultCreateOptions() metav1.CreateOptions { return metav1.CreateOptions{} }

// DefaultUpdateOptions are the default options for UPDATE requests.
func DefaultUpdateOptions() metav1.UpdateOptions { return metav1.UpdateOptions{} }

func init() {
	gardenSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		gardencorescheme.AddToScheme,
	)
	utilruntime.Must(gardenSchemeBuilder.AddToScheme(GardenScheme))

	seedSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		dnsv1alpha1.AddToScheme,
		gardenextensionsscheme.AddToScheme,
		resourcesscheme.AddToScheme,
		autoscalingscheme.AddToScheme,
		hvpav1alpha1.AddToScheme,
		druidv1alpha1.AddToScheme,
		apiextensionsscheme.AddToScheme,
	)
	utilruntime.Must(seedSchemeBuilder.AddToScheme(SeedScheme))

	shootSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		apiextensionsscheme.AddToScheme,
		apiregistrationscheme.AddToScheme,
		autoscalingscheme.AddToScheme,
	)
	utilruntime.Must(shootSchemeBuilder.AddToScheme(ShootScheme))

	plantSchemeBuilder := runtime.NewSchemeBuilder(
		corescheme.AddToScheme,
		gardencorescheme.AddToScheme,
	)
	utilruntime.Must(plantSchemeBuilder.AddToScheme(PlantScheme))
}

// MergeFunc determines how oldOj is merged into new oldObj.
type MergeFunc func(newObj, oldObj *unstructured.Unstructured)

// Applier is an interface which describes declarative operations to apply multiple
// Kubernetes objects.
type Applier interface {
	ApplyManifest(ctx context.Context, unstructured UnstructuredReader, options MergeFuncs) error
	DeleteManifest(ctx context.Context, unstructured UnstructuredReader, opts ...DeleteManifestOption) error
}

// Interface is used to wrap the interactions with a Kubernetes cluster
// (which are performed with the help of kubernetes/client-go) in order to allow the implementation
// of several Kubernetes versions.
type Interface interface {
	RESTConfig() *rest.Config
	RESTMapper() meta.RESTMapper
	RESTClient() rest.Interface

	// Client returns the ClientSet's controller-runtime client. This client should be used by default, as it carries
	// a cache, which uses SharedIndexInformers to keep up-to-date.
	Client() client.Client
	// DirectClient returns a controller-runtime client, which can be used to talk to the API server directly
	// (without using a cache).
	DirectClient() client.Client
	// Cache returns the ClientSet's controller-runtime cache. It can be used to get Informers for arbitrary objects.
	Cache() cache.Cache

	// Applier returns an Applier which uses the ClientSet's client.
	Applier() Applier
	// ChartRenderer returns a ChartRenderer populated with the cluster's Capabilities.
	ChartRenderer() chartrenderer.Interface
	// ChartApplier returns a ChartApplier using the ClientSet's ChartRenderer and Applier.
	ChartApplier() ChartApplier

	Kubernetes() kubernetesclientset.Interface
	GardenCore() gardencoreclientset.Interface
	APIExtension() apiextensionsclientset.Interface
	APIRegistration() apiregistrationclientset.Interface

	// Deprecated: Use `Client()` and utils instead.
	ForwardPodPort(string, string, int, int) (chan struct{}, error)
	CheckForwardPodPort(string, string, int, int) error

	// Version returns the server version of the targeted Kubernetes cluster.
	Version() string
	// DiscoverVersion tries to retrieve the server version of the targeted Kubernetes cluster and updates the
	// ClientSet's saved version accordingly. Use Version if you only want to retrieve the kubernetes version instead
	// of refreshing the ClientSet's saved version.
	DiscoverVersion() (*version.Info, error)

	// Start starts the cache of the ClientSet's controller-runtime client and returns immediately.
	// It must be called first before using the client to retrieve objects from the API server.
	Start(stopCh <-chan struct{})
	// WaitForCacheSync waits for the cache of the ClientSet's controller-runtime client to be synced.
	WaitForCacheSync(stopCh <-chan struct{}) bool
}
