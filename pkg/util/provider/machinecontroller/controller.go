// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"slices"
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

	"github.com/Masterminds/semver/v3"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesinformers "k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	policyv1listers "k8s.io/client-go/listers/policy/v1"
	storagelisters "k8s.io/client-go/listers/storage/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// MCMFinalizerName is the finalizer used to tag dependencies before deletion
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
	targetCoreInformerFactory kubernetesinformers.SharedInformerFactory,
	secretInformer coreinformers.SecretInformer,
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
		machineTerminationQueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinetermination"),
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
	if targetCoreInformerFactory != nil {
		controller.pvcLister = targetCoreInformerFactory.Core().V1().PersistentVolumeClaims().Lister()
		controller.pvLister = targetCoreInformerFactory.Core().V1().PersistentVolumes().Lister()
		controller.volumeAttachementLister = targetCoreInformerFactory.Storage().V1().VolumeAttachments().Lister()
		controller.nodeLister = targetCoreInformerFactory.Core().V1().Nodes().Lister()
		controller.podLister = targetCoreInformerFactory.Core().V1().Pods().Lister()
		controller.pdbLister = targetCoreInformerFactory.Policy().V1().PodDisruptionBudgets().Lister()
	}
	controller.secretLister = secretInformer.Lister()
	controller.machineClassLister = machineClassInformer.Lister()
	controller.machineLister = machineInformer.Lister()

	// Controller syncs
	if targetCoreInformerFactory != nil {
		controller.pvcSynced = targetCoreInformerFactory.Core().V1().PersistentVolumeClaims().Informer().HasSynced
		controller.pvSynced = targetCoreInformerFactory.Core().V1().PersistentVolumes().Informer().HasSynced
		controller.volumeAttachementSynced = targetCoreInformerFactory.Storage().V1().VolumeAttachments().Informer().HasSynced
		controller.nodeSynced = targetCoreInformerFactory.Core().V1().Nodes().Informer().HasSynced
		controller.podSynced = targetCoreInformerFactory.Core().V1().Pods().Informer().HasSynced
		controller.pdbSynced = targetCoreInformerFactory.Policy().V1().PodDisruptionBudgets().Informer().HasSynced
	}
	controller.secretSynced = secretInformer.Informer().HasSynced
	controller.machineClassSynced = machineClassInformer.Informer().HasSynced
	controller.machineSynced = machineInformer.Informer().HasSynced

	// Secret Controller's Informers
	_, _ = secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.secretAdd,
		DeleteFunc: controller.secretDelete,
	})

	_, _ = machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassToSecretAdd,
		UpdateFunc: controller.machineClassToSecretUpdate,
		DeleteFunc: controller.machineClassToSecretDelete,
	})

	// Machine Class Controller's Informers
	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineToMachineClassAdd,
		DeleteFunc: controller.machineToMachineClassDelete,
	})

	_, _ = machineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineClassAdd,
		UpdateFunc: controller.machineClassUpdate,
		DeleteFunc: controller.machineClassDelete,
	})

	// Machine Controller's Informers
	if targetCoreInformerFactory != nil {
		_, _ = targetCoreInformerFactory.Core().V1().Nodes().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.addNodeToMachine,
			UpdateFunc: controller.updateNodeToMachine,
			DeleteFunc: controller.deleteNodeToMachine,
		})
	}

	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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
	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// updateMachineToSafety makes sure that orphan VM handler is invoked on some specific machine obj updates
		UpdateFunc: controller.updateMachineToSafety,
		// deleteMachineToSafety makes sure that orphan VM handler is invoked on any machine deletion
		DeleteFunc: controller.deleteMachineToSafety,
	})

	// Drain Controller's Informers
	if targetCoreClient != nil && targetCoreInformerFactory != nil && k8sutils.IsResourceSupported(
		targetCoreClient,
		schema.GroupResource{
			Group:    k8sutils.VolumeAttachmentGroupName,
			Resource: k8sutils.VolumeAttachmentResourceName,
		},
	) {
		controller.volumeAttachmentHandler = drain.NewVolumeAttachmentHandler()
		_, _ = targetCoreInformerFactory.Storage().V1().VolumeAttachments().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

	// control clients
	controlMachineClient machineapi.MachineV1alpha1Interface
	controlCoreClient    kubernetes.Interface
	// target clients – nil when running without a target cluster
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

	// control listers
	secretLister       corelisters.SecretLister
	machineClassLister machinelisters.MachineClassLister
	machineLister      machinelisters.MachineLister
	// target listers – nil when running without a target cluster
	pvcLister               corelisters.PersistentVolumeClaimLister
	pvLister                corelisters.PersistentVolumeLister
	nodeLister              corelisters.NodeLister
	pdbLister               policyv1listers.PodDisruptionBudgetLister
	volumeAttachementLister storagelisters.VolumeAttachmentLister
	podLister               corelisters.PodLister
	// queues
	secretQueue                 workqueue.RateLimitingInterface
	nodeQueue                   workqueue.RateLimitingInterface
	machineClassQueue           workqueue.RateLimitingInterface
	machineQueue                workqueue.RateLimitingInterface
	machineTerminationQueue     workqueue.RateLimitingInterface
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
	podSynced               cache.InformerSynced
}

func (dc *controller) Run(workers int, stopCh <-chan struct{}) {

	var (
		waitGroup sync.WaitGroup
	)

	defer runtimeutil.HandleCrash()
	defer dc.permitGiver.Close()
	defer dc.nodeQueue.ShutDown()
	defer dc.secretQueue.ShutDown()
	defer dc.machineClassQueue.ShutDown()
	defer dc.machineQueue.ShutDown()
	defer dc.machineTerminationQueue.ShutDown()
	defer dc.machineSafetyOrphanVMsQueue.ShutDown()
	defer dc.machineSafetyAPIServerQueue.ShutDown()

	syncedFuncs := []cache.InformerSynced{
		dc.secretSynced,
		dc.pvcSynced,
		dc.pvSynced,
		dc.volumeAttachementSynced,
		dc.nodeSynced,
		dc.machineClassSynced,
		dc.machineSynced,
	}
	if dc.targetKubernetesVersion != nil && k8sutils.ConstraintK8sGreaterEqual121.Check(dc.targetKubernetesVersion) {
		syncedFuncs = append(syncedFuncs, dc.pdbSynced)
	}
	// filter out nil funcs (disabled target cluster)
	syncedFuncs = slices.DeleteFunc(syncedFuncs, func(fn cache.InformerSynced) bool { return fn == nil })

	if !cache.WaitForCacheSync(stopCh, syncedFuncs...) {
		runtimeutil.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	klog.V(1).Info("Starting machine-controller-manager")

	// The controller implement the prometheus.Collector interface and can therefore
	// be passed to the metrics registry. Collectors which added to the registry
	// will collect metrics to expose them via the metrics endpoint of the mcm
	// every time when the endpoint is called.
	prometheus.MustRegister(dc)

	for range workers {
		worker.Run(dc.secretQueue, "ClusterSecret", worker.DefaultMaxRetries, true, dc.reconcileClusterSecretKey, stopCh, &waitGroup)
		worker.Run(dc.machineClassQueue, "ClusterMachineClass", worker.DefaultMaxRetries, true, dc.reconcileClusterMachineClassKey, stopCh, &waitGroup)
		worker.Run(dc.machineQueue, "ClusterMachine", worker.DefaultMaxRetries, true, dc.reconcileClusterMachineKey, stopCh, &waitGroup)
		worker.Run(dc.machineTerminationQueue, "ClusterMachineTermination", worker.DefaultMaxRetries, true, dc.reconcileClusterMachineTermination, stopCh, &waitGroup)
		worker.Run(dc.machineSafetyOrphanVMsQueue, "ClusterMachineSafetyOrphanVMs", worker.DefaultMaxRetries, true, dc.reconcileClusterMachineSafetyOrphanVMs, stopCh, &waitGroup)

		// don't start these controllers if running without a target cluster
		if dc.targetCoreClient != nil {
			worker.Run(dc.nodeQueue, "ClusterNode", worker.DefaultMaxRetries, true, dc.reconcileClusterNodeKey, stopCh, &waitGroup)
			worker.Run(dc.machineSafetyAPIServerQueue, "ClusterMachineAPIServer", worker.DefaultMaxRetries, true, dc.reconcileClusterMachineSafetyAPIServer, stopCh, &waitGroup)
		}
	}

	<-stopCh
	klog.V(1).Info("Shutting down Machine Controller Manager ")
	handlers.UpdateHealth(false)

	waitGroup.Wait()
}
