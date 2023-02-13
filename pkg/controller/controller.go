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
	coreinformers "k8s.io/client-go/informers/core/v1"
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
	nodeInformer coreinformers.NodeInformer,
	machineInformer machineinformers.MachineInformer,
	machineSetInformer machineinformers.MachineSetInformer,
	machineDeploymentInformer machineinformers.MachineDeploymentInformer,
	recorder record.EventRecorder,
	safetyOptions options.SafetyOptions,
	autoscalerScaleDownAnnotationDuringRollout bool,
) (Controller, error) {
	controller := &controller{
		namespace:                      namespace,
		controlMachineClient:           controlMachineClient,
		controlCoreClient:              controlCoreClient,
		targetCoreClient:               targetCoreClient,
		recorder:                       recorder,
		expectations:                   NewUIDTrackingContExpectations(NewContExpectations()),
		nodeQueue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		machineQueue:                   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		machineSetQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineset"),
		machineDeploymentQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinedeployment"),
		machineSafetyOvershootingQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinesafetyovershooting"),
		safetyOptions:                  safetyOptions,
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
	controller.nodeLister = nodeInformer.Lister()
	controller.machineLister = machineInformer.Lister()
	controller.machineSetLister = machineSetInformer.Lister()
	controller.machineDeploymentLister = machineDeploymentInformer.Lister()

	// Controller syncs
	controller.nodeSynced = nodeInformer.Informer().HasSynced
	controller.machineSynced = machineInformer.Informer().HasSynced
	controller.machineSetSynced = machineSetInformer.Informer().HasSynced
	controller.machineDeploymentSynced = machineDeploymentInformer.Informer().HasSynced

	// MachineSet Controller Informers
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineToMachineSet,
		UpdateFunc: controller.updateMachineToMachineSet,
		DeleteFunc: controller.deleteMachineToMachineSet,
	})

	machineSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueMachineSet,
		UpdateFunc: controller.machineSetUpdate,
		DeleteFunc: controller.enqueueMachineSet,
	})

	// MachineDeployment Controller Informers
	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.deleteMachineDeployment,
	})

	machineSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineSetToDeployment,
		UpdateFunc: controller.updateMachineSetToDeployment,
		DeleteFunc: controller.deleteMachineSetToDeployment,
	})

	machineDeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addMachineDeployment,
		UpdateFunc: controller.updateMachineDeployment,
		DeleteFunc: controller.deleteMachineDeployment,
	})

	// MachineSafety Controller Informers

	// We follow the kubernetes way of reconciling the safety controller
	// done by adding empty key objects. We initialize it, to trigger
	// running of different safety loop on MCM startup.
	controller.machineSafetyOvershootingQueue.Add("")

	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

	controlMachineClient machineapi.MachineV1alpha1Interface
	controlCoreClient    kubernetes.Interface
	targetCoreClient     kubernetes.Interface

	recorder          record.EventRecorder
	machineControl    MachineControlInterface
	machineSetControl MachineSetControlInterface
	safetyOptions     options.SafetyOptions
	expectations      *UIDTrackingContExpectations

	internalExternalScheme *runtime.Scheme
	// listers
	nodeLister              corelisters.NodeLister
	machineLister           machinelisters.MachineLister
	machineSetLister        machinelisters.MachineSetLister
	machineDeploymentLister machinelisters.MachineDeploymentLister
	// queues
	nodeQueue                      workqueue.RateLimitingInterface
	machineQueue                   workqueue.RateLimitingInterface
	machineSetQueue                workqueue.RateLimitingInterface
	machineDeploymentQueue         workqueue.RateLimitingInterface
	machineSafetyOvershootingQueue workqueue.RateLimitingInterface
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

	if !cache.WaitForCacheSync(stopCh, c.nodeSynced, c.machineSynced, c.machineSetSynced, c.machineDeploymentSynced) {
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
		worker.Run(c.machineSetQueue, "ClusterMachineSet", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSet, stopCh, &waitGroup)
		worker.Run(c.machineDeploymentQueue, "ClusterMachineDeployment", worker.DefaultMaxRetries, true, c.reconcileClusterMachineDeployment, stopCh, &waitGroup)
		worker.Run(c.machineSafetyOvershootingQueue, "ClusterMachineSafetyOvershooting", worker.DefaultMaxRetries, true, c.reconcileClusterMachineSafetyOvershooting, stopCh, &waitGroup)
	}

	<-stopCh
	klog.V(1).Info("Shutting down Machine Controller Manager ")
	handlers.UpdateHealth(false)

	waitGroup.Wait()
}
