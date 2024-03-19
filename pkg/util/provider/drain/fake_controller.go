// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package drain is used to drain nodes
package drain

import (
	"time"

	customfake "github.com/gardener/machine-controller-manager/pkg/fakeclient"
	"k8s.io/apimachinery/pkg/runtime"
	coreinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
)

func createFakeController(
	stop <-chan struct{},
	namespace string,
	targetCoreObjects []runtime.Object,
) (
	kubernetes.Interface,
	corelisters.PersistentVolumeLister,
	corelisters.PersistentVolumeClaimLister,
	corelisters.NodeLister,
	func() bool,
	func() bool,
	func() bool,
	*customfake.FakeObjectTracker) {

	fakeTargetCoreClient, targetCoreObjectTracker := customfake.NewCoreClientSet(targetCoreObjects...)
	go targetCoreObjectTracker.Start()

	coreTargetInformerFactory := coreinformers.NewFilteredSharedInformerFactory(
		fakeTargetCoreClient,
		100*time.Millisecond,
		namespace,
		nil,
	)
	defer coreTargetInformerFactory.Start(stop)
	coreTargetSharedInformers := coreTargetInformerFactory.Core().V1()
	pvcs := coreTargetSharedInformers.PersistentVolumeClaims()
	pvs := coreTargetSharedInformers.PersistentVolumes()
	nodes := coreTargetSharedInformers.Nodes()

	pvcLister := pvcs.Lister()
	pvLister := pvs.Lister()
	nodeLister := nodes.Lister()

	pvcSynced := pvcs.Informer().HasSynced
	pvSynced := pvs.Informer().HasSynced
	nodeSynced := nodes.Informer().HasSynced

	return fakeTargetCoreClient, pvLister, pvcLister, nodeLister, pvcSynced, pvSynced, nodeSynced, targetCoreObjectTracker
}
