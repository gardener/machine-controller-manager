// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"slices"
	"sync"

	machineinternal "github.com/gardener/machine-controller-manager/pkg/apis/machine"
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machinescheme "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/scheme"
	machineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	machineinformers "github.com/gardener/machine-controller-manager/pkg/client/informers/externalversions/machine/v1alpha1"
	machinelisters "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/handlers"
	"github.com/gardener/machine-controller-manager/pkg/options"
	"github.com/gardener/machine-controller-manager/pkg/util/worker"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	kubernetesinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	// DeleteFinalizerName is the finalizer used to identify the controller acting on an object
	DeleteFinalizerName = "machine.sapcloud.io/machine-controller-manager"
)

// NewController returns a new Node controller.
func NewController(
	namespace string,
	controlMachineClient machineapi.MachineV1alpha1Interface,
	controlCoreClient kubernetes.Interface,
	targetCoreClient kubernetes.Interface,
	targetCoreInformerFactory kubernetesinformers.SharedInformerFactory,
	machineInformer machineinformers.MachineInformer,
	machineSetInformer machineinformers.MachineSetInformer,
	machineDeploymentInformer machineinformers.MachineDeploymentInformer,
	recorder record.EventRecorder,
	safetyOptions options.SafetyOptions,
	autoscalerScaleDownAnnotationDuringRollout bool,
) (Controller, error) {
	controller := &controller{
		namespace:            namespace,
		controlMachineClient: controlMachineClient,
		controlCoreClient:    controlCoreClient,
		targetCoreClient:     targetCoreClient,
		recorder:             recorder,
		expectations:         NewUIDTrackingContExpectations(NewContExpectations()),
		nodeQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "node"},
		),
		machineQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "machine"},
		),
		machineSetQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "machineset"},
		),
		machineDeploymentQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "machinedeployment"},
		),
		machineSafetyOvershootingQueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "machinesafetyovershooting"},
		),
		safetyOptions: safetyOptions,
		autoscalerScaleDownAnnotationDuringRollout: autoscalerScaleDownAnnotationDuringRollout,
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

	controller.machineControl = RealMachineControl{
		controlMachineClient: controlMachineClient,
		Recorder:             eventBroadcaster.NewRecorder(machinescheme.Scheme, corev1.EventSource{Component: "machineset-controller"}),
	}

	controller.machineSetControl = RealMachineSetControl{
		controlMachineClient: controlMachineClient,
		Recorder:             eventBroadcaster.NewRecorder(machinescheme.Scheme, corev1.EventSource{Component: "machinedeployment-controller"}),
	}

	// Controller listers
	if targetCoreInformerFactory != nil {
		controller.nodeLister = targetCoreInformerFactory.Core().V1().Nodes().Lister()
	}
	controller.machineLister = machineInformer.Lister()
	controller.machineSetLister = machineSetInformer.Lister()
	controller.machineDeploymentLister = machineDeploymentInformer.Lister()

	// Controller syncs
	if targetCoreInformerFactory != nil {
		controller.nodeSynced = targetCoreInformerFactory.Core().V1().Nodes().Informer().HasSynced
	}
	controller.machineSynced = machineInformer.Informer().HasSynced
	controller.machineSetSynced = machineSetInformer.Informer().HasSynced
	controller.machineDeploymentSynced = machineDeploymentInformer.Informer().HasSynced

	// MachineSet Controller Informers
	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineToMachineSet,
		UpdateFunc: controller.updateMachineToMachineSet,
		DeleteFunc: controller.deleteMachineToMachineSet,
	})

	_, _ = machineSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueMachineSet,
		UpdateFunc: controller.machineSetUpdate,
		DeleteFunc: controller.enqueueMachineSet,
	})

	// MachineDeployment Controller Informers
	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: controller.updateMachineToMachineDeployment,
		DeleteFunc: controller.deleteMachineDeployment,
	})

	_, _ = machineSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineSetToDeployment,
		UpdateFunc: controller.updateMachineSetToDeployment,
		DeleteFunc: controller.deleteMachineSetToDeployment,
	})

	_, _ = machineDeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineDeployment,
		UpdateFunc: controller.updateMachineDeployment,
		DeleteFunc: controller.deleteMachineDeployment,
	})

	// MachineSafety Controller Informers

	// We follow the kubernetes way of reconciling the safety controller
	// done by adding empty key objects. We initialize it, to trigger
	// running of different safety loop on MCM startup.
	controller.machineSafetyOvershootingQueue.Add("")

	_, _ = machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		// addMachineToSafetyOvershooting makes sure machine objects does not overshoot
		AddFunc: controller.addMachineToSafetyOvershooting,
	})

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
	namespace                                  string
	autoscalerScaleDownAnnotationDuringRollout bool

	// control clients
	controlMachineClient machineapi.MachineV1alpha1Interface
	controlCoreClient    kubernetes.Interface
	// target clients – nil when running without a target cluster
	targetCoreClient kubernetes.Interface

	recorder          record.EventRecorder
	machineControl    MachineControlInterface
	machineSetControl MachineSetControlInterface
	safetyOptions     options.SafetyOptions
	expectations      *UIDTrackingContExpectations

	internalExternalScheme *runtime.Scheme
	// control listers
	machineLister           machinelisters.MachineLister
	machineSetLister        machinelisters.MachineSetLister
	machineDeploymentLister machinelisters.MachineDeploymentLister
	// target listers – nil when running without a target cluster
	nodeLister corelisters.NodeLister
	// queues
	nodeQueue                      workqueue.TypedRateLimitingInterface[string]
	machineQueue                   workqueue.TypedRateLimitingInterface[string]
	machineSetQueue                workqueue.TypedRateLimitingInterface[string]
	machineDeploymentQueue         workqueue.TypedRateLimitingInterface[string]
	machineSafetyOvershootingQueue workqueue.TypedRateLimitingInterface[string]
	// syncs
	nodeSynced              cache.InformerSynced
	machineSynced           cache.InformerSynced
	machineSetSynced        cache.InformerSynced
	machineDeploymentSynced cache.InformerSynced
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {

	var (
		waitGroup sync.WaitGroup
	)

	defer runtimeutil.HandleCrash()
	defer c.nodeQueue.ShutDown()
	defer c.machineQueue.ShutDown()
	defer c.machineSetQueue.ShutDown()
	defer c.machineDeploymentQueue.ShutDown()
	defer c.machineSafetyOvershootingQueue.ShutDown()

	syncedFuncs := []cache.InformerSynced{
		c.nodeSynced,
		c.machineSynced,
		c.machineSetSynced,
		c.machineDeploymentSynced,
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
	prometheus.MustRegister(c)

	for range workers {
		worker.Run(c.machineSetQueue, "ClusterMachineSet", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSet, stopCh, &waitGroup)
		worker.Run(c.machineDeploymentQueue, "ClusterMachineDeployment", worker.DefaultMaxRetries, true, c.reconcileClusterMachineDeployment, stopCh, &waitGroup)
		worker.Run(c.machineSafetyOvershootingQueue, "ClusterMachineSafetyOvershooting", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSafetyOvershooting, stopCh, &waitGroup)
	}

	<-stopCh
	klog.V(1).Info("Shutting down Machine Controller Manager ")
	handlers.UpdateHealth(false)

	waitGroup.Wait()
}
