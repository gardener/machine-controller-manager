// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by informer-gen. DO NOT EDIT.

package internalversion

import (
	"context"
	time "time"

	machine "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	clientsetinternalversion "github.com/gardener/machine-controller-manager/pkg/client/clientset/internalversion"
	internalinterfaces "github.com/gardener/machine-controller-manager/pkg/client/informers/internalversion/internalinterfaces"
	internalversion "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/internalversion"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// MachineDeploymentInformer provides access to a shared informer and lister for
// MachineDeployments.
type MachineDeploymentInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() internalversion.MachineDeploymentLister
}

type machineDeploymentInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewMachineDeploymentInformer constructs a new informer for MachineDeployment type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewMachineDeploymentInformer(client clientsetinternalversion.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredMachineDeploymentInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredMachineDeploymentInformer constructs a new informer for MachineDeployment type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredMachineDeploymentInformer(client clientsetinternalversion.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Machine().MachineDeployments(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.Machine().MachineDeployments(namespace).Watch(context.TODO(), options)
			},
		},
		&machine.MachineDeployment{},
		resyncPeriod,
		indexers,
	)
}

func (f *machineDeploymentInformer) defaultInformer(client clientsetinternalversion.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredMachineDeploymentInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *machineDeploymentInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&machine.MachineDeployment{}, f.defaultInformer)
}

func (f *machineDeploymentInformer) Lister() internalversion.MachineDeploymentLister {
	return internalversion.NewMachineDeploymentLister(f.Informer().GetIndexer())
}
