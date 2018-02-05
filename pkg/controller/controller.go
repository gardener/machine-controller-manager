/*
Copyright 2017 The Gardener Authors.

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
package controller

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/api/core/v1"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"

	nodeclientset "github.com/gardener/node-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	nodeinformers "github.com/gardener/node-controller-manager/pkg/client/informers/externalversions/machine/v1alpha1"
	nodelisters "github.com/gardener/node-controller-manager/pkg/client/listers/machine/v1alpha1"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	nodescheme "github.com/gardener/node-controller-manager/pkg/client/clientset/versioned/scheme"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries                = 15
	pollingStartInterval      = 1 * time.Second
	pollingMaxBackoffDuration = 1 * time.Hour

	ClassAnnotation     = "machine.sapcloud.io/class"
	MachineIDAnnotation = "machine.sapcloud.io/id"
	DeleteFinalizerName = "machine.sapcloud.io/node-controller-manager"
)

// NewController returns a new Node controller.
func NewController(
	kubeClient kubernetes.Interface,
	nodeClient nodeclientset.MachineV1alpha1Interface,
	secretInformer coreinformers.SecretInformer,
	nodeInformer coreinformers.NodeInformer,
	awsMachineClassInformer nodeinformers.AWSMachineClassInformer,
	azureMachineClassInformer nodeinformers.AzureMachineClassInformer,
	gcpMachineClassInformer nodeinformers.GCPMachineClassInformer,
	machineInformer nodeinformers.MachineInformer,
	machineSetInformer nodeinformers.MachineSetInformer,
	machineDeploymentInformer nodeinformers.MachineDeploymentInformer,
	recorder record.EventRecorder,
) (Controller, error) {
	controller := &controller{
		kubeClient:             kubeClient,
		nodeClient:             nodeClient,
		recorder:               recorder,
		expectations:           NewUIDTrackingControllerExpectations(NewControllerExpectations()),
		secretQueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret"),
		nodeQueue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		nodeToMachineQueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodeToMachine"),
		awsMachineClassQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "awsmachineclass"),
		azureMachineClassQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "azuremachineclass"),
		gcpMachineClassQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "gcpmachineclass"),
		machineQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machine"),
		machineSetQueue:        workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machineset"),
		machineDeploymentQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "machinedeployment"),
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})

	controller.machineControl = RealMachineControl{
		NodeClient: nodeClient,
		Recorder:   eventBroadcaster.NewRecorder(nodescheme.Scheme, v1.EventSource{Component: "machineset-controller"}),
	}

	controller.machineSetControl = RealMachineSetControl{
		NodeClient: nodeClient,
		Recorder:   eventBroadcaster.NewRecorder(nodescheme.Scheme, v1.EventSource{Component: "machinedeployment-controller"}),
	}

	// Controller listers
	controller.secretLister = secretInformer.Lister()
	controller.awsMachineClassLister = awsMachineClassInformer.Lister()
	controller.azureMachineClassLister = azureMachineClassInformer.Lister()
	controller.gcpMachineClassLister = gcpMachineClassInformer.Lister()
	controller.nodeLister = nodeInformer.Lister()
	controller.machineLister = machineInformer.Lister()
	controller.machineSetLister = machineSetInformer.Lister()
	controller.machineDeploymentLister = machineDeploymentInformer.Lister()

	// Controller syncs
	controller.secretSynced = secretInformer.Informer().HasSynced
	controller.awsMachineClassSynced = awsMachineClassInformer.Informer().HasSynced
	controller.azureMachineClassSynced = azureMachineClassInformer.Informer().HasSynced
	controller.gcpMachineClassSynced = gcpMachineClassInformer.Informer().HasSynced
	controller.nodeSynced = nodeInformer.Informer().HasSynced
	controller.machineSynced = machineInformer.Informer().HasSynced
	controller.machineSetSynced = machineSetInformer.Informer().HasSynced
	controller.machineDeploymentSynced = machineDeploymentInformer.Informer().HasSynced

	/*
		secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.secretAdd,
			UpdateFunc: controller.secretUpdate,
		})*/

	// Aws Controller Informers
	awsMachineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.awsMachineClassAdd,
		UpdateFunc: controller.awsMachineClassUpdate,
		DeleteFunc: controller.awsMachineClassDelete,
	})

	// Azure Controller Informers
	azureMachineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.azureMachineClassAdd,
		UpdateFunc: controller.azureMachineClassUpdate,
		DeleteFunc: controller.azureMachineClassDelete,
	})

	// GCP Controller Informers
	gcpMachineClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.gcpMachineClassAdd,
		UpdateFunc: controller.gcpMachineClassUpdate,
		DeleteFunc: controller.gcpMachineClassDelete,
	})

	// Node Controller Informers
	nodeInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				//node := obj.(*apicorev1.Node)
				return true //metav1.HasAnnotation(node.ObjectMeta, ClassAnnotation)
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc:    controller.nodeAdd,
				UpdateFunc: controller.nodeUpdate,
				DeleteFunc: controller.nodeDelete,
			},
		})

	// Machine Controller Informers
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.nodeToMachineAdd,
		UpdateFunc: controller.nodeToMachineUpdate,
		DeleteFunc: controller.nodeToMachineDelete,
	})

	machineInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.machineAdd,
		UpdateFunc: controller.machineUpdate,
		DeleteFunc: controller.machineDelete,
	})

	// MachineSet Controller informers
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

	// MachineDeployment Controller informers
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
	kubeClient kubernetes.Interface
	nodeClient nodeclientset.MachineV1alpha1Interface

	// listers
	secretLister            corelisters.SecretLister
	nodeLister              corelisters.NodeLister
	awsMachineClassLister   nodelisters.AWSMachineClassLister
	azureMachineClassLister nodelisters.AzureMachineClassLister
	gcpMachineClassLister   nodelisters.GCPMachineClassLister
	machineLister           nodelisters.MachineLister
	machineSetLister        nodelisters.MachineSetLister
	machineDeploymentLister nodelisters.MachineDeploymentLister

	recorder record.EventRecorder

	// A TTLCache of pod creates/deletes each rc expects to see.
	expectations *UIDTrackingControllerExpectations

	// Control Interfaces.
	machineControl    MachineControlInterface
	machineSetControl MachineSetControlInterface

	// queues
	secretQueue            workqueue.RateLimitingInterface
	nodeQueue              workqueue.RateLimitingInterface
	nodeToMachineQueue     workqueue.RateLimitingInterface
	awsMachineClassQueue   workqueue.RateLimitingInterface
	azureMachineClassQueue workqueue.RateLimitingInterface
	gcpMachineClassQueue   workqueue.RateLimitingInterface
	machineQueue           workqueue.RateLimitingInterface
	machineSetQueue        workqueue.RateLimitingInterface
	machineDeploymentQueue workqueue.RateLimitingInterface

	// syncs
	secretSynced            cache.InformerSynced
	nodeSynced              cache.InformerSynced
	awsMachineClassSynced   cache.InformerSynced
	azureMachineClassSynced cache.InformerSynced
	gcpMachineClassSynced   cache.InformerSynced
	machineSynced           cache.InformerSynced
	machineSetSynced        cache.InformerSynced
	machineDeploymentSynced cache.InformerSynced
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtimeutil.HandleCrash()
	defer c.nodeQueue.ShutDown()
	defer c.awsMachineClassQueue.ShutDown()
	defer c.machineQueue.ShutDown()
	defer c.machineSetQueue.ShutDown()
	defer c.machineDeploymentQueue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.secretSynced, c.nodeSynced, c.awsMachineClassSynced, c.machineSynced, c.machineSetSynced, c.machineDeploymentSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	glog.Info("Starting Node-controller-manager")
	var waitGroup sync.WaitGroup

	for i := 0; i < workers; i++ {
		createWorker(c.awsMachineClassQueue, "ClusterAWSMachineClass", maxRetries, true, c.reconcileClusterAWSMachineClassKey, stopCh, &waitGroup)
		createWorker(c.azureMachineClassQueue, "ClusterAzureMachineClass", maxRetries, true, c.reconcileClusterAzureMachineClassKey, stopCh, &waitGroup)
		createWorker(c.gcpMachineClassQueue, "ClusterGCPMachineClass", maxRetries, true, c.reconcileClusterGCPMachineClassKey, stopCh, &waitGroup)

		createWorker(c.nodeQueue, "ClusterNode", maxRetries, true, c.reconcileClusterNodeKey, stopCh, &waitGroup)

		createWorker(c.machineQueue, "ClusterMachine", maxRetries, true, c.reconcileClusterMachineKey, stopCh, &waitGroup)
		createWorker(c.nodeToMachineQueue, "ClusterNodeToMachine", maxRetries, true, c.reconcileClusterNodeToMachineKey, stopCh, &waitGroup)

		createWorker(c.machineSetQueue, "ClusterMachineSet", maxRetries, true, c.reconcileClusterMachineSet, stopCh, &waitGroup)

		createWorker(c.machineDeploymentQueue, "ClusterMachineDeployment", maxRetries, true, c.syncMachineDeployment, stopCh, &waitGroup)
	}

	<-stopCh
	glog.Info("Shutting down node controller manager ")

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
					glog.V(4).Infof("Error syncing %s %v: %v", resourceType, key, err)
					queue.AddRateLimited(key)
					return false
				}

				glog.V(4).Infof("Dropping %s %q out of the queue: %v", resourceType, key, err)
				queue.Forget(key)
				return false
			}()
		}
	}
}
