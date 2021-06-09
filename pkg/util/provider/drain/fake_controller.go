/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

*/

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
