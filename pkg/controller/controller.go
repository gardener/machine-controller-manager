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

	nodeclientset "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/clientset/typed/node/v1alpha1"
	nodeinformers "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/informers/externalversions/node/v1alpha1"
	nodelisters "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/listers/node/v1alpha1"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"

	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	nodescheme "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/clientset/scheme"

)

const (
	maxRetries                = 15
	pollingStartInterval      = 1 * time.Second
	pollingMaxBackoffDuration = 1 * time.Hour

	ClassAnnotation = "node.sapcloud.io/class"

	InstanceIDAnnotation = "node.sapcloud.io/id"

	DeleteFinalizerName = "node.sapcloud.io/operator"
)

// NewController returns a new Node controller.
func NewController(
	kubeClient kubernetes.Interface,
	nodeClient nodeclientset.NodeV1alpha1Interface,
	secretInformer coreinformers.SecretInformer,
	nodeInformer coreinformers.NodeInformer,
	awsInstanceClassInformer nodeinformers.AWSInstanceClassInformer,
	instanceInformer nodeinformers.InstanceInformer,
	instanceSetInformer nodeinformers.InstanceSetInformer,
	instanceDeploymentInformer nodeinformers.InstanceDeploymentInformer,
	recorder record.EventRecorder,
) (Controller, error) {
	controller := &controller{
		kubeClient:     		kubeClient,
		nodeClient:     		nodeClient,
		recorder:       		recorder,
		expectations:     		NewUIDTrackingControllerExpectations(NewControllerExpectations()),
		secretQueue:   	 		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "secret"),
		nodeQueue:      		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "node"),
		nodeToInstanceQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "nodeToInstance"),
		awsInstanceClassQueue: 	workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instanceclass"),
		instanceQueue:  		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instance"),
		instanceSetQueue:		workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instanceset"),
		instanceDeploymentQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "instancedeployment"),
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})

	controller.instanceControl = RealInstanceControl {
			NodeClient: nodeClient,
			Recorder:   eventBroadcaster.NewRecorder(nodescheme.Scheme, v1.EventSource{Component: "instanceset-controller"}),
	}

	controller.instanceSetControl = RealISControl { 
			NodeClient: nodeClient,
			Recorder:	eventBroadcaster.NewRecorder(nodescheme.Scheme, v1.EventSource{Component: "instancedeployment-controller"}),
	}

	// Controller listers
	controller.secretLister = secretInformer.Lister()
	controller.awsInstanceClassLister = awsInstanceClassInformer.Lister()
	controller.nodeLister = nodeInformer.Lister()
	controller.instanceLister = instanceInformer.Lister()	
	controller.instanceSetLister = instanceSetInformer.Lister()
	controller.instanceDeploymentLister = instanceDeploymentInformer.Lister()
	
	// Controller syncs
	controller.secretSynced = secretInformer.Informer().HasSynced
	controller.awsInstanceClassSynced = awsInstanceClassInformer.Informer().HasSynced
	controller.nodeSynced = nodeInformer.Informer().HasSynced
	controller.instanceSynced = instanceInformer.Informer().HasSynced
	controller.instanceSetSynced = instanceSetInformer.Informer().HasSynced
	controller.instanceDeploymentSynced = instanceDeploymentInformer.Informer().HasSynced

	/*
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.secretAdd,
		UpdateFunc: controller.secretUpdate,
	})*/

	// Aws Controller Informers
	awsInstanceClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.awsInstanceClassAdd,
		UpdateFunc: controller.awsInstanceClassUpdate,
		DeleteFunc: controller.awsInstanceClassDelete,
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

	// Instance Controller Informers
	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.nodeToInstanceAdd,
		UpdateFunc: controller.nodeToInstanceUpdate,
		DeleteFunc: controller.nodeToInstanceDelete,
	})

	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.instanceAdd,
		UpdateFunc: controller.instanceUpdate,
		DeleteFunc: controller.instanceDelete,
	})

	// InstanceSet Controller informers
	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addInstanceToInstanceSet,
		UpdateFunc: controller.updateInstanceToInstanceSet,
		DeleteFunc: controller.deleteInstanceToInstanceSet,
	})
	
	instanceSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.enqueueInstanceSet,
		UpdateFunc: controller.instanceSetUpdate,
		DeleteFunc: controller.enqueueInstanceSet,
	})

	// InstanceDeployment Controller informers
	instanceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: controller.deleteInstanceDeployment,
	})

	instanceSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addInstanceSetToDeployment,
		UpdateFunc: controller.updateInstanceSetToDeployment,
		DeleteFunc: controller.deleteInstanceSetToDeployment,
	})

	instanceDeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:	controller.addInstanceDeployment, 
		UpdateFunc: controller.updateInstanceDeployment, 
		DeleteFunc: controller.deleteInstanceDeployment, 
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
	kubeClient 				kubernetes.Interface
	nodeClient 				nodeclientset.NodeV1alpha1Interface

	// listers
	secretLister    		 corelisters.SecretLister
	nodeLister      		 corelisters.NodeLister
	awsInstanceClassLister 	 nodelisters.AWSInstanceClassLister
	instanceLister  		 nodelisters.InstanceLister
	instanceSetLister  		 nodelisters.InstanceSetLister
	instanceDeploymentLister nodelisters.InstanceDeploymentLister
	
	recorder 				record.EventRecorder

	// A TTLCache of pod creates/deletes each rc expects to see.
	expectations    		*UIDTrackingControllerExpectations

	// Control Interfaces.
	instanceControl 		InstanceControlInterface
	instanceSetControl      ISControlInterface


	// queues
	secretQueue    			  workqueue.RateLimitingInterface
	nodeQueue      			  workqueue.RateLimitingInterface
	nodeToInstanceQueue  	  workqueue.RateLimitingInterface
	awsInstanceClassQueue 	  workqueue.RateLimitingInterface
	instanceQueue  			  workqueue.RateLimitingInterface
	instanceSetQueue  		  workqueue.RateLimitingInterface
	instanceDeploymentQueue   workqueue.RateLimitingInterface

	// syncs
	secretSynced    		  cache.InformerSynced
	nodeSynced      		  cache.InformerSynced
	awsInstanceClassSynced 	  cache.InformerSynced
	instanceSynced  		  cache.InformerSynced
	instanceSetSynced  		  cache.InformerSynced
	instanceDeploymentSynced  cache.InformerSynced
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtimeutil.HandleCrash()
	defer c.nodeQueue.ShutDown()
	defer c.awsInstanceClassQueue.ShutDown()
	defer c.instanceQueue.ShutDown()
	defer c.instanceSetQueue.ShutDown()
	defer c.instanceDeploymentQueue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.secretSynced, c.nodeSynced, c.awsInstanceClassSynced, c.instanceSynced, c.instanceSetSynced, c.instanceDeploymentSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	glog.Info("Starting Node-controller-manager")
	var waitGroup sync.WaitGroup

	for i := 0; i < workers; i++ {
		createWorker(c.awsInstanceClassQueue, "ClusterawsInstanceClass", maxRetries, true, c.reconcileClusterawsInstanceClassKey, stopCh, &waitGroup)
		
		createWorker(c.nodeQueue, "ClusterNode", maxRetries, true, c.reconcileClusterNodeKey, stopCh, &waitGroup)
		
		createWorker(c.instanceQueue, "ClusterInstance", maxRetries, true, c.reconcileClusterInstanceKey, stopCh, &waitGroup)
		createWorker(c.nodeToInstanceQueue, "ClusterNodeToInstance", maxRetries, true, c.reconcileClusterNodeToInstanceKey, stopCh, &waitGroup)
		
		createWorker(c.instanceSetQueue, "ClusterInstanceSet", maxRetries, true, c.syncInstanceSet, stopCh, &waitGroup)

		createWorker(c.instanceDeploymentQueue, "ClusterInstanceDeployment", maxRetries, true, c.syncInstanceDeployment, stopCh, &waitGroup)
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
