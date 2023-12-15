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

	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/util/k8sutils"
	"github.com/gardener/machine-controller-manager/pkg/util/permits"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/drain"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/options"
	"github.com/gardener/machine-controller-manager/pkg/util/worker"

	machineinternal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions/machine/v1alpha1"
	machinelisters "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"

	"github.com/Masterminds/semver"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	policyv1informers "k8s.io/client-go/informers/policy/v1"
	policyv1beta1informers "k8s.io/client-go/informers/policy/v1beta1"
	storageinformers "k8s.io/client-go/informers/storage/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	policyv1beta1listers "k8s.io/client-go/listers/policy/v1beta1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
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
	pdbV1beta1Informer policyv1beta1informers.PodDisruptionBudgetInformer,
	pdbV1Informer policyv1informers.PodDisruptionBudgetInformer,
	volumeAttachmentInformer storageinformers.VolumeAttachmentInformer,
	machineClassInformer machineinformers.MachineClassInformer,
	machineInformer machineinformers.MachineInformer,
	recorder record.EventRecorder,
	safetyOptions options.SafetyOptions,
	nodeConditions string,
	bootstrapTokenAuthExtraGroups string,
	targetKubernetesVersion *semver.Version,
) (Controller, error) {
	const (
		permitGiverStaleEntryTimeout = 1 * time.Hour
		// janitorFreq is the time after which permitGiver ranges its map for stale entries
		janitorFreq = 10 * time.Minute
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
		permitGiver:                   permits.NewPermitGiver(permitGiverStaleEntryTimeout, janitorFreq),
		targetKubernetesVersion:       targetKubernetesVersion,
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
	controller.volumeAttachementLister = volumeAttachmentInformer.Lister()
	controller.machineClassLister = machineClassInformer.Lister()
	controller.nodeLister = nodeInformer.Lister()
	controller.machineLister = machineInformer.Lister()

	// Controller syncs
	controller.pvcSynced = pvcInformer.Informer().HasSynced
	controller.pvSynced = pvInformer.Informer().HasSynced
	controller.secretSynced = secretInformer.Informer().HasSynced
	controller.volumeAttachementSynced = volumeAttachmentInformer.Informer().HasSynced
	controller.machineClassSynced = machineClassInformer.Informer().HasSynced
	controller.nodeSynced = nodeInformer.Informer().HasSynced
	controller.machineSynced = machineInformer.Informer().HasSynced

	if k8sutils.ConstraintK8sGreaterEqual121.Check(targetKubernetesVersion) {
		controller.pdbV1Lister = pdbV1Informer.Lister()
		controller.pdbV1Synced = pdbV1Informer.Informer().HasSynced
	} else {
		controller.pdbV1beta1Lister = pdbV1beta1Informer.Lister()
		controller.pdbV1beta1Synced = pdbV1beta1Informer.Informer().HasSynced
	}

	// Secret Controller's Informers
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.secretAdd,
		DeleteFunc: controller.secretDelete,
	})

	machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassToSecretAdd,
		UpdateFunc: controller.machineClassToSecretUpdate,
		DeleteFunc: controller.machineClassToSecretDelete,
	})

	// Machine Class Controller's Informers
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineToMachineClassAdd,
		DeleteFunc: controller.machineToMachineClassDelete,
	})

	machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassAdd,
		UpdateFunc: controller.machineClassUpdate,
		DeleteFunc: controller.machineClassDelete,
	})

	// Machine Controller's Informers
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

	// MachineSafety Controller's Informers
	// We follow the kubernetes way of reconciling the safety controller
	// done by adding empty key objects. We initialize it, to trigger
	// running of different safety loop on MCM startup.
	controller.machineSafetyOrphanVMsQueue.Add("")
	controller.machineSafetyAPIServerQueue.Add("")
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// updateMachineToSafety makes sure that orphan VM handler is invoked on some specific machine obj updates
		UpdateFunc: controller.updateMachineToSafety,
		// deleteMachineToSafety makes sure that orphan VM handler is invoked on any machine deletion
		DeleteFunc: controller.deleteMachineToSafety,
	})

	// Drain Controller's Informers
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

	controlMachineClient    machineapi.MachineV1alpha1Interface
	controlCoreClient       kubernetes.Interface
	targetCoreClient        kubernetes.Interface
	targetKubernetesVersion *semver.Version

	recorder                record.EventRecorder
	safetyOptions           options.SafetyOptions
	internalExternalScheme  *runtime.Scheme
	driver                  driver.Driver
	volumeAttachmentHandler *drain.VolumeAttachmentHandler
	// permitGiver store two things:
	// - mutex per machinedeployment
	// - lastAcquire time
	// it is used to limit removal of `health timed out` machines
	permitGiver permits.PermitGiver

	// listers
	pvcLister               corelisters.PersistentVolumeClaimLister
	pvLister                corelisters.PersistentVolumeLister
	secretLister            corelisters.SecretLister
	nodeLister              corelisters.NodeLister
	pdbV1beta1Lister        policyv1beta1listers.PodDisruptionBudgetLister
	pdbV1Lister             policyv1listers.PodDisruptionBudgetLister
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
	pdbV1beta1Synced        cache.InformerSynced
	pdbV1Synced             cache.InformerSynced
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
	defer c.permitGiver.Close()
	defer c.nodeQueue.ShutDown()
	defer c.secretQueue.ShutDown()
	defer c.machineClassQueue.ShutDown()
	defer c.machineQueue.ShutDown()
	defer c.machineSafetyOrphanVMsQueue.ShutDown()
	defer c.machineSafetyAPIServerQueue.ShutDown()

	if k8sutils.ConstraintK8sGreaterEqual121.Check(c.targetKubernetesVersion) {
		if !cache.WaitForCacheSync(stopCh, c.secretSynced, c.pvcSynced, c.pvSynced, c.pdbV1Synced, c.volumeAttachementSynced, c.nodeSynced, c.machineClassSynced, c.machineSynced) {
			runtimeutil.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
			return
		}
	} else {
		if !cache.WaitForCacheSync(stopCh, c.secretSynced, c.pvcSynced, c.pvSynced, c.pdbV1beta1Synced, c.volumeAttachementSynced, c.nodeSynced, c.machineClassSynced, c.machineSynced) {
			runtimeutil.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
			return
		}
	}

	klog.V(1).Info("Starting machine-controller-manager")

	// The controller implement the prometheus.Collector interface and can therefore
	// be passed to the metrics registry. Collectors which added to the registry
	// will collect metrics to expose them via the metrics endpoint of the mcm
	// every time when the endpoint is called.
	prometheus.MustRegister(c)

	for i := 0; i < workers; i++ {
		worker.Run(c.secretQueue, "ClusterSecret", worker.DefaultMaxRetries, true, c.reconcileClusterSecretKey, stopCh, &waitGroup)
		worker.Run(c.machineClassQueue, "ClusterMachineClass", worker.DefaultMaxRetries, true, c.reconcileClusterMachineClassKey, stopCh, &waitGroup)
		worker.Run(c.nodeQueue, "ClusterNode", worker.DefaultMaxRetries, true, c.reconcileClusterNodeKey, stopCh, &waitGroup)
		worker.Run(c.machineQueue, "ClusterMachine", worker.DefaultMaxRetries, true, c.reconcileClusterMachineKey, stopCh, &waitGroup)
		worker.Run(c.machineSafetyOrphanVMsQueue, "ClusterMachineSafetyOrphanVMs", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSafetyOrphanVMs, stopCh, &waitGroup)
		worker.Run(c.machineSafetyAPIServerQueue, "ClusterMachineAPIServer", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSafetyAPIServer, stopCh, &waitGroup)
	}

	<-stopCh
	klog.V(1).Info("Shutting down Machine Controller Manager ")
	handlers.UpdateHealth(false)

	waitGroup.Wait()
}
