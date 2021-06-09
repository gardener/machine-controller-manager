/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"sync"
	"time"

	machineinternal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions/machine/v1alpha1"
	machinelisters "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/util/k8sutils"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/options"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyinformers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	policylisters "k8s.io/client-go/listers/policy/v1beta1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

const (
	maxRetries = 15

	// ClassAnnotation is the annotation used to identify a machine class
	ClassAnnotation = "machine.sapcloud.io/class"
	// MachineIDAnnotation is the annotation used to identify a machine ID
	MachineIDAnnotation = "machine.sapcloud.io/id"
	// MCMFinalizerName is the finalizer used to tag dependecies before deletion
	// of the object. This finalizer is carried over from the MCM
	MCMFinalizerName = "machine.sapcloud.io/machine-controller-manager"
	// MCFinalizerName is the finalizer created for the external
	// machine controller to differentiate it from the MCMFinalizerName
	// This finalizer is added only on secret-objects to avoid race between in-tree and out-of-tree controllers.
	// This is a stopgap solution to resolve: https://github.com/gardener/machine-controller-manager/issues/486.
	MCFinalizerName = "machine.sapcloud.io/machine-controller"
)

// NewController returns a new Node controller.
func NewController(
	namespace string,
	controlMachineClient machineapi.MachineV1alpha1Interface,
	controlCoreClient kubernetes.Interface,
	targetCoreClient kubernetes.Interface,
	driver driver.Driver,
	pvcInformer coreinformers.PersistentVolumeClaimInformer,
	pvInformer coreinformers.PersistentVolumeInformer,
	secretInformer coreinformers.SecretInformer,
	nodeInformer coreinformers.NodeInformer,
	pdbInformer policyinformers.PodDisruptionBudgetInformer,
	volumeAttachmentInformer storageinformers.VolumeAttachmentInformer,
	machineClassInformer machineinformers.MachineClassInformer,
	machineInformer machineinformers.MachineInformer,
	recorder record.EventRecorder,
	safetyOptions options.SafetyOptions,
	nodeConditions string,
	bootstrapTokenAuthExtraGroups string,
) (Controller, error) {
	const (
		// volumeAttachmentGroupName group name
		volumeAttachmentGroupName = "storage.k8s.io"
		// volumenAttachmentKind is the kind used for VolumeAttachment
		volumeAttachmentResourceName = "volumeattachments"
		// volumeAttachmentResource is the kind used for VolumeAttachment
		volumeAttachmentResourceKind = "VolumeAttachment"
	)

	controller := &controller{
		namespace:                     namespace,
		controlMachineClient:          controlMachineClient,
		controlCoreClient:             controlCoreClient,
		targetCoreClient:              targetCoreClient,
		recorder:                      recorder,
		secretQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret"),
		nodeQueue:                     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		machineClassQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineclass"),
		machineQueue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		machineSafetyOrphanVMsQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyorphanvms"),
		machineSafetyAPIServerQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyapiserver"),
		safetyOptions:                 safetyOptions,
		nodeConditions:                nodeConditions,
		driver:                        driver,
		bootstrapTokenAuthExtraGroups: bootstrapTokenAuthExtraGroups,
		volumeAttachmentHandler:       nil,
	}

	controller.internalExternalScheme = runtime.NewScheme()

	if err := machineinternal.AddToScheme(controller.internalExternalScheme); err != nil {
		return nil, err
	}

	if err := machinev1alpha1.AddToScheme(controller.internalExternalScheme); err != nil {
		return nil, err
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: typedcorev1.New(controlCoreClient.CoreV1().RESTClient()).Events(namespace)})

	// Controller listers
	controller.pvcLister = pvcInformer.Lister()
	controller.pvLister = pvInformer.Lister()
	controller.secretLister = secretInformer.Lister()
	controller.pdbLister = pdbInformer.Lister()
	// TODO: Need to handle K8s versions below 1.13 differently
	controller.volumeAttachementLister = volumeAttachmentInformer.Lister()
	controller.machineClassLister = machineClassInformer.Lister()
	controller.nodeLister = nodeInformer.Lister()
	controller.machineLister = machineInformer.Lister()

	// Controller syncs
	controller.pvcSynced = pvcInformer.Informer().HasSynced
	controller.pvSynced = pvInformer.Informer().HasSynced
	controller.secretSynced = secretInformer.Informer().HasSynced
	controller.pdbSynced = pdbInformer.Informer().HasSynced
	controller.volumeAttachementSynced = volumeAttachmentInformer.Informer().HasSynced
	controller.machineClassSynced = machineClassInformer.Informer().HasSynced
	controller.nodeSynced = nodeInformer.Informer().HasSynced
	controller.machineSynced = machineInformer.Informer().HasSynced

	// Secret Controller Informers
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.secretAdd,
		DeleteFunc: controller.secretDelete,
	})

	machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassToSecretAdd,
		UpdateFunc: controller.machineClassToSecretUpdate,
		DeleteFunc: controller.machineClassToSecretDelete,
	})

	// Machine Class Controller Informers
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineToMachineClassAdd,
		UpdateFunc: controller.machineToMachineClassUpdate,
		DeleteFunc: controller.machineToMachineClassDelete,
	})

	machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassAdd,
		UpdateFunc: controller.machineClassUpdate,
		DeleteFunc: controller.machineClassDelete,
	})

	// Machine Controller Informers
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addNodeToMachine,
		UpdateFunc: controller.updateNodeToMachine,
		DeleteFunc: controller.deleteNodeToMachine,
	})

	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachine,
		UpdateFunc: controller.updateMachine,
		DeleteFunc: controller.deleteMachine,
	})

	// MachineSafety Controller Informers
	// We follow the kubernetes way of reconciling the safety controller
	// done by adding empty key objects. We initialize it, to trigger
	// running of different safety loop on MCM startup.
	controller.machineSafetyOrphanVMsQueue.Add("")
	controller.machineSafetyAPIServerQueue.Add("")
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// deleteMachineToSafety makes sure that orphan VM handler is invoked
		DeleteFunc: controller.deleteMachineToSafety,
	})

	// Drain Controller Informers
	if k8sutils.IsResourceSupported(
		targetCoreClient,
		schema.GroupResource{
			Group:    k8sutils.VolumeAttachmentGroupName,
			Resource: k8sutils.VolumeAttachmentResourceName,
		},
	) {
		controller.volumeAttachmentHandler = drain.NewVolumeAttachmentHandler()
		volumeAttachmentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.volumeAttachmentHandler.AddVolumeAttachment,
			UpdateFunc: controller.volumeAttachmentHandler.UpdateVolumeAttachment,
		})
	}

	return controller, nil
}

// Controller describes a controller for
type Controller interface {
	// Run runs the controller until the given stop channel can be read from.
	// workers specifies the number of goroutines, per resource, processing work
	// from the resource workqueues
	Run(workers int, stopCh <-chan struct{})
}

// controller is a concrete Controller.
type controller struct {
	namespace                     string
	nodeConditions                string
	bootstrapTokenAuthExtraGroups string

	controlMachineClient machineapi.MachineV1alpha1Interface
	controlCoreClient    kubernetes.Interface
	targetCoreClient     kubernetes.Interface

	recorder                record.EventRecorder
	safetyOptions           options.SafetyOptions
	internalExternalScheme  *runtime.Scheme
	driver                  driver.Driver
	volumeAttachmentHandler *drain.VolumeAttachmentHandler

	// listers
	pvcLister               corelisters.PersistentVolumeClaimLister
	pvLister                corelisters.PersistentVolumeLister
	secretLister            corelisters.SecretLister
	nodeLister              corelisters.NodeLister
	pdbLister               policylisters.PodDisruptionBudgetLister
	volumeAttachementLister storagelisters.VolumeAttachmentLister
	machineClassLister      machinelisters.MachineClassLister
	machineLister           machinelisters.MachineLister
	// queues
	secretQueue                 workqueue.RateLimitingInterface
	nodeQueue                   workqueue.RateLimitingInterface
	machineClassQueue           workqueue.RateLimitingInterface
	machineQueue                workqueue.RateLimitingInterface
	machineSafetyOrphanVMsQueue workqueue.RateLimitingInterface
	machineSafetyAPIServerQueue workqueue.RateLimitingInterface
	// syncs
	pvcSynced               cache.InformerSynced
	pvSynced                cache.InformerSynced
	secretSynced            cache.InformerSynced
	pdbSynced               cache.InformerSynced
	volumeAttachementSynced cache.InformerSynced
	nodeSynced              cache.InformerSynced
	machineClassSynced      cache.InformerSynced
	machineSynced           cache.InformerSynced
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {

	var (
		waitGroup sync.WaitGroup
	)

	defer runtimeutil.HandleCrash()
	defer c.nodeQueue.ShutDown()
	defer c.secretQueue.ShutDown()
	defer c.machineClassQueue.ShutDown()
	defer c.machineQueue.ShutDown()
	defer c.machineSafetyOrphanVMsQueue.ShutDown()
	defer c.machineSafetyAPIServerQueue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.secretSynced, c.pvcSynced, c.pvSynced, c.pdbSynced, c.volumeAttachementSynced, c.nodeSynced, c.machineClassSynced, c.machineSynced) {
		runtimeutil.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	klog.V(1).Info("Starting machine-controller-manager")

	// The controller implement the prometheus.Collector interface and can therefore
	// be passed to the metrics registry. Collectors which added to the registry
	// will collect metrics to expose them via the metrics endpoint of the mcm
	// every time when the endpoint is called.
	prometheus.MustRegister(c)

	for i := 0; i < workers; i++ {
		createWorker(c.secretQueue, "ClusterSecret", maxRetries, true, c.reconcileClusterSecretKey, stopCh, &waitGroup)
		createWorker(c.machineClassQueue, "ClusterMachineClass", maxRetries, true, c.reconcileClusterMachineClassKey, stopCh, &waitGroup)
		createWorker(c.nodeQueue, "ClusterNode", maxRetries, true, c.reconcileClusterNodeKey, stopCh, &waitGroup)
		createWorker(c.machineQueue, "ClusterMachine", maxRetries, true, c.reconcileClusterMachineKey, stopCh, &waitGroup)
		createWorker(c.machineSafetyOrphanVMsQueue, "ClusterMachineSafetyOrphanVMs", maxRetries, true, c.reconcileClusterMachineSafetyOrphanVMs, stopCh, &waitGroup)
		createWorker(c.machineSafetyAPIServerQueue, "ClusterMachineAPIServer", maxRetries, true, c.reconcileClusterMachineSafetyAPIServer, stopCh, &waitGroup)
	}

	<-stopCh
	klog.V(1).Info("Shutting down Machine Controller Manager ")
	handlers.UpdateHealth(false)

	waitGroup.Wait()
}

// createWorker creates and runs a worker thread that just processes items in the
// specified queue. The worker will run until stopCh is closed. The worker will be
// added to the wait group when started and marked done when finished.
func createWorker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, forgetAfterSuccess bool, reconciler func(key string) error, stopCh <-chan struct{}, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	go func() {
		wait.Until(worker(queue, resourceType, maxRetries, forgetAfterSuccess, reconciler), time.Second, stopCh)
		waitGroup.Done()
	}()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// If reconciler returns an error, requeue the item up to maxRetries before giving up.
// It enforces that the reconciler is never invoked concurrently with the same key.
// If forgetAfterSuccess is true, it will cause the queue to forget the item should reconciliation
// have no error.
func worker(queue workqueue.RateLimitingInterface, resourceType string, maxRetries int, forgetAfterSuccess bool, reconciler func(key string) error) func() {
	return func() {
		exit := false
		for !exit {
			exit = func() bool {
				key, quit := queue.Get()
				if quit {
					return true
				}
				defer queue.Done(key)

				err := reconciler(key.(string))
				if err == nil {
					if forgetAfterSuccess {
						queue.Forget(key)
					}
					return false
				}

				if queue.NumRequeues(key) < maxRetries {
					klog.V(4).Infof("Error syncing %s %v: %v", resourceType, key, err)
					queue.AddRateLimited(key)
					return false
				}

				klog.V(4).Infof("Dropping %s %q out of the queue: %v", resourceType, key, err)
				queue.Forget(key)
				return false
			}()
		}
	}
}
