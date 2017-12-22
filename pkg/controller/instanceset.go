/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/replicaset/replica_set.go

Modifications Copyright 2017 The Gardener Authors.
*/
package controller

import (
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"
	"errors"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/integer"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	
	"github.com/golang/glog"

	"github.com/gardener/node-controller-manager/pkg/apis/node"
	"github.com/gardener/node-controller-manager/pkg/apis/node/validation"
	"github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
)

const (
	// Realistic value of the burstReplica field for the instance set manager based off
	// performance requirements for kubernetes 1.0.
	BurstReplicas = 100

	// The number of times we retry updating a ReplicaSet's status.
	statusUpdateRetries = 1

	// Kind for the instanceSet
	instanceSetKind = "InstanceSet"

)

var controllerKindIS = v1alpha1.SchemeGroupVersion.WithKind("InstanceSet")


// getInstanceInstanceSets returns the InstanceSets matching the given Instance.
func (c *controller) getInstanceInstanceSets(instance *v1alpha1.Instance) ([]*v1alpha1.InstanceSet, error) {
	
	if len(instance.Labels) == 0 {
		err := errors.New("No InstanceSets found for instance because it has no labels")
		glog.V(4).Info(err, ": ", instance.Name)
		return nil, err
	}

	list, err := c.instanceSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	
	var iss []*v1alpha1.InstanceSet
	for _, is := range list {
		if is.Namespace != instance.Namespace {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(is.Spec.Selector)
		if err != nil {
			glog.Errorf("Invalid selector: %v", err)
			return nil, err
		}

		// If a ReplicaSet with a nil or empty selector creeps in, it should match nothing, not everything.
		if selector.Empty() || !selector.Matches(labels.Set(instance.Labels)) {
			continue
		}
		iss = append(iss, is)
		//glog.Info("D", len(iss))
	}

	if len(iss) == 0 {
		err := errors.New("No InstanceSets found for instance doesn't have matching labels")
		glog.V(4).Info(err, ": ", instance.Name)
		return nil, err	
	}

	return iss, nil
}


// resolveInstanceSetControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (c *controller) resolveInstanceSetControllerRef(namespace string, controllerRef *metav1.OwnerReference) *v1alpha1.InstanceSet {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != instanceSetKind { //TOCheck
		return nil
	}
	is, err := c.instanceSetLister.Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if is.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return is
}

// callback when InstanceSet is updated
func (c *controller) instanceSetUpdate(old, cur interface{}) {
	oldIS := old.(*v1alpha1.InstanceSet)
	curIS := cur.(*v1alpha1.InstanceSet)

	// You might imagine that we only really need to enqueue the
	// instance set when Spec changes, but it is safer to sync any
	// time this function is triggered. That way a full informer
	// resync can requeue any instance set that don't yet have instances
	// but whose last attempts at creating a instance have failed (since
	// we don't block on creation of instances) instead of those
	// instance sets stalling indefinitely. Enqueueing every time
	// does result in some spurious syncs (like when Status.Replica
	// is updated and the watch notification from it retriggers
	// this function), but in general extra resyncs shouldn't be
	// that bad as ReplicaSets that haven't met expectations yet won't
	// sync, and all the listing is done using local stores.
	if oldIS.Spec.Replicas != curIS.Spec.Replicas {
		glog.V(4).Infof("%v %v updated. Desired instance count change: %d->%d", curIS.Name, oldIS.Spec.Replicas, curIS.Spec.Replicas)
	}
	c.enqueueInstanceSet(curIS)
}

// When a instance is created, enqueue the instance set that manages it and update its expectations.
func (c *controller) addInstanceToInstanceSet(obj interface{}) {
	instance := obj.(*v1alpha1.Instance)

	if instance.DeletionTimestamp != nil {
		// on a restart of the controller manager, it's possible a new instance shows up in a state that
		// is already pending deletion. Prevent the instance from being a creation observation.
		c.deleteInstanceToInstanceSet(instance)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(instance); controllerRef != nil {
		is := c.resolveInstanceSetControllerRef(instance.Namespace, controllerRef)
		if is == nil {
			return
		}
		isKey, err := KeyFunc(is)
		if err != nil {
			return
		}
		glog.V(4).Infof("Instance %s created: %#v.", instance.Name, instance)
		c.expectations.CreationObserved(isKey)
		c.enqueueInstanceSet(is)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching ReplicaSets and sync
	// them to see if anyone wants to adopt it.
	// DO NOT observe creation because no controller should be waiting for an
	// orphan.
	iss, err := c.getInstanceInstanceSets(instance)
	if err != nil{
		return
	} else if len(iss) == 0 {
		return
	}

	glog.V(4).Infof("Orphan Instance %s created: %#v.", instance.Name, instance)
	for _, is := range iss {
		c.enqueueInstanceSet(is)
	}
}

// When a instance is updated, figure out what instance set/s manage it and wake them
// up. If the labels of the instance have changed we need to awaken both the old
// and new instance set. old and cur must be *v1alpha1.Instance types.
func (c *controller) updateInstanceToInstanceSet(old, cur interface{}) {
	curInst := cur.(*v1alpha1.Instance)
	oldInst := old.(*v1alpha1.Instance)
	if curInst.ResourceVersion == oldInst.ResourceVersion {
		// Periodic resync will send update events for all known instances.
		// Two different versions of the same instance will always have different RVs.
		return
	}

	labelChanged := !reflect.DeepEqual(curInst.Labels, oldInst.Labels)
	if curInst.DeletionTimestamp != nil {
		// when a instance is deleted gracefully it's deletion timestamp is first modified to reflect a grace period,
		// and after such time has passed, the kubelet actually deletes it from the store. We receive an update
		// for modification of the deletion timestamp and expect an rs to create more replicas asap, not wait
		// until the kubelet actually deletes the instance. This is different from the Phase of a instance changing, because
		// an rs never initiates a phase change, and so is never asleep waiting for the same.
		c.deleteInstanceToInstanceSet(curInst)
		if labelChanged {
			// we don't need to check the oldInstance.DeletionTimestamp because DeletionTimestamp cannot be unset.
			c.deleteInstanceToInstanceSet(oldInst)
		}
		return
	}


	curControllerRef := metav1.GetControllerOf(curInst)
	oldControllerRef := metav1.GetControllerOf(oldInst)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if is := c.resolveInstanceSetControllerRef(oldInst.Namespace, oldControllerRef); is != nil {
			c.enqueueInstanceSet(is)
		}
	}

	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		is := c.resolveInstanceSetControllerRef(curInst.Namespace, curControllerRef)
		if is == nil {
			return
		}
		glog.V(4).Infof("Instance %s updated, objectMeta %+v -> %+v.", curInst.Name, oldInst.ObjectMeta, curInst.ObjectMeta)
		c.enqueueInstanceSet(is)
		// TODO: MinReadySeconds in the Instance will generate an Available condition to be added in
		// the Instance status which in turn will trigger a requeue of the owning instance set thus
		// having its status updated with the newly available replica. For now, we can fake the
		// update by resyncing the controller MinReadySeconds after the it is requeued because
		// a Instance transitioned to Ready.
		// Note that this still suffers from #29229, we are just moving the problem one level
		// "closer" to kubelet (from the deployment to the instance set controller).
		/*
		if !isInstanceReady(oldInst) && instanceutil.IsInstanceReady(curInst) && is.Spec.MinReadySeconds > 0 {
			glog.V(2).Infof("%v %q will be enqueued after %ds for availability check", rsc.Kind, rs.Name, rs.Spec.MinReadySeconds)
			// Add a second to avoid milliseconds skew in AddAfter.
			// See https://github.com/kubernetes/kubernetes/issues/39785#issuecomment-279959133 for more info.
			c.enqueueReplicaSetAfter(is, (time.Duration(rs.Spec.MinReadySeconds)*time.Second)+time.Second)
		}*/
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	if labelChanged || controllerRefChanged {
		iss, err := c.getInstanceInstanceSets(curInst)
		if err != nil{
			return
		} else if len(iss) == 0 {
			return
		}
		glog.V(4).Infof("Orphan Instance %s updated, objectMeta %+v -> %+v.", curInst.Name, oldInst.ObjectMeta, curInst.ObjectMeta)
		for _, is := range iss {
			c.enqueueInstanceSet(is)
		}
	}

}

// When a instance is deleted, enqueue the instance set that manages the instance and update its expectations.
// obj could be an *v1alpha1.Instance, or a DeletionFinalStateUnknown marker item.
func (c *controller) deleteInstanceToInstanceSet(obj interface{}) {
	instance, ok := obj.(*v1alpha1.Instance)

	// When a delete is dropped, the relist will notice a instance in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the instance
	// changed labels the new ReplicaSet will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		instance, ok = tombstone.Obj.(*v1alpha1.Instance)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a instance %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(instance)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	is := c.resolveInstanceSetControllerRef(instance.Namespace, controllerRef)
	if is == nil {
		return
	}
	isKey, err := KeyFunc(is)
	if err != nil {
		return
	}
	glog.V(4).Infof("Instance %s/%s deleted through %v, timestamp %+v: %#v.", instance.Namespace, instance.Name, utilruntime.GetCaller(), instance.DeletionTimestamp, instance)
	c.expectations.DeletionObserved(isKey, InstanceKey(instance))
	c.enqueueInstanceSet(is)
}

// obj could be an *extensions.ReplicaSet, or a DeletionFinalStateUnknown marker item.
func (c *controller) enqueueInstanceSet(obj interface{}) {
	key, err := KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.instanceSetQueue.Add(key)
}

// obj could be an *extensions.ReplicaSet, or a DeletionFinalStateUnknown marker item.
func (c *controller) enqueueInstanceSetAfter(obj interface{}, after time.Duration) {
	key, err := KeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}
	c.instanceSetQueue.AddAfter(key, after)
}

// manageReplicas checks and updates replicas for the given ReplicaSet.
// Does NOT modify <filteredInstances>.
// It will requeue the instance set in case of an error while creating/deleting instances.
func (c *controller) manageReplicas(allInstances []*v1alpha1.Instance, is *v1alpha1.InstanceSet) error {
	
	isKey, err := KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for %v %#v: %v", is.Kind, is, err))
		return nil
	}

	var activeInstances, staleInstances []*v1alpha1.Instance
	for _, instance := range allInstances {
		if IsInstanceActive(instance) {
			//glog.Info("Active instance: ", instance.Name)
			activeInstances = append(activeInstances, instance)
		} else if IsInstanceFailed(instance) {
			staleInstances = append(staleInstances, instance)
		}
	}

	if len(staleInstances) >= 1 {
		glog.V(2).Infof("Deleting stales")
	}
	c.terminateInstances(staleInstances, is)
	
	diff := len(activeInstances) - int((is.Spec.Replicas))
	if diff < 0 {
		//glog.Info("Start Create:", diff)
		diff *= -1
		if diff > BurstReplicas {
			diff = BurstReplicas
		}
		// TODO: Track UIDs of creates just like deletes. The problem currently
		// is we'd need to wait on the result of a create to record the instance's
		// UID, which would require locking *across* the create, which will turn
		// into a performance bottleneck. We should generate a UID for the instance
		// beforehand and store it via ExpectCreations.
		c.expectations.ExpectCreations(isKey, diff)
		glog.V(1).Infof("Too few replicas for InstanceSet %s, need %d, creating %d", is.Name, (is.Spec.Replicas), diff)
		// Batch the instance creates. Batch sizes start at SlowStartInitialBatchSize
		// and double with each successful iteration in a kind of "slow start".
		// This handles attempts to start large numbers of instances that would
		// likely all fail with the same error. For example a project with a
		// low quota that attempts to create a large number of instances will be
		// prevented from spamming the API service with the instance create requests
		// after one of its instances fails.  Conveniently, this also prevents the
		// event spam that those failures would generate.
		successfulCreations, err := slowStartBatch(diff, SlowStartInitialBatchSize, func() error {
			boolPtr := func(b bool) *bool { return &b }
			controllerRef := &metav1.OwnerReference{
				APIVersion:         controllerKindIS.GroupVersion().String(), //#ToCheck
				Kind:               controllerKindIS.Kind, //is.Kind,
				Name:               is.Name,
				UID:                is.UID,
				BlockOwnerDeletion: boolPtr(true),
				Controller:         boolPtr(true),
			}
			//glog.Info("Printing InstanceSet details ... %v", &is)
			err := c.instanceControl.CreateInstancesWithControllerRef(&is.Spec.Template, is, controllerRef)
			if err != nil && apierrors.IsTimeout(err) {
				// Instance is created but its initialization has timed out.
				// If the initialization is successful eventually, the
				// controller will observe the creation via the informer.
				// If the initialization fails, or if the instance keeps
				// uninitialized for a long time, the informer will not
				// receive any update, and the controller will create a new
				// instance when the expectation expires.
				return nil
			}
			return err
		})
		//glog.Info("Stop Create:", diff)

		// Any skipped instances that we never attempted to start shouldn't be expected.
		// The skipped instances will be retried later. The next controller resync will
		// retry the slow start process.
		if skippedInstances := diff - successfulCreations; skippedInstances > 0 {
			glog.V(2).Infof("Slow-start failure. Skipping creation of %d instances, decrementing expectations for %v %v/%v", skippedInstances, is.Kind, is.Namespace, is.Name)
			for i := 0; i < skippedInstances; i++ {
				// Decrement the expected number of creates because the informer won't observe this instance
				c.expectations.CreationObserved(isKey)
			}
		}
		return err
	} else if diff > 0 {
		if diff > BurstReplicas {
			diff = BurstReplicas
		}
		glog.V(2).Infof("Too many replicas for %v %s/%s, need %d, deleting %d", is.Kind, is.Namespace, is.Name, (is.Spec.Replicas), diff)

		instancesToDelete := getInstancesToDelete(activeInstances, diff)

		// Snapshot the UIDs (ns/name) of the instances we're expecting to see
		// deleted, so we know to record their expectations exactly once either
		// when we see it as an update of the deletion timestamp, or as a delete.
		// Note that if the labels on a instance/rs change in a way that the instance gets
		// orphaned, the rs will only wake up after the expectations have
		// expired even if other instances are deleted.
		c.expectations.ExpectDeletions(isKey, getInstanceKeys(instancesToDelete))

		c.terminateInstances(instancesToDelete, is)
	}
	
	return nil
}

// syncReplicaSet will sync the ReplicaSet with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its instances created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (c *controller) syncInstanceSet(key string) error {

	startTime := time.Now()
	defer func() {
		glog.V(4).Infof("Finished syncing %q (%v)", key, time.Since(startTime))
	}()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	is, err := c.instanceSetLister.Get(name)
	//time.Sleep(10 * time.Second)
	//glog.V(2).Infof("2.. Printing Key : %v , Printing InstanceSet First :: %+v", key, is)
	if apierrors.IsNotFound(err) {
		glog.V(4).Infof("%v has been deleted", key)
		c.expectations.DeleteExpectations(key)
		return nil
	}
	if err != nil {
		return err
	}

	// Validate InstanceSet
	internalInstanceSet := &node.InstanceSet{}
	err = api.Scheme.Convert(is, internalInstanceSet, nil)
	if err != nil {
		return err
	}
	validationerr := validation.ValidateInstanceSet(internalInstanceSet)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of InstanceSet failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	AWSInstanceClass, err := c.awsInstanceClassLister.Get(is.Spec.Template.Spec.Class.Name)
	if err != nil {
		glog.V(2).Infof("AWSInstanceClass for InstanceSet %q not found %q. Skipping. %v", is.Name, is.Spec.Template.Spec.Class.Name, err)
		return nil
	}

	// Validate AWSInstanceClass
	internalAWSInstanceClass := &node.AWSInstanceClass{}
	err = api.Scheme.Convert(AWSInstanceClass, internalAWSInstanceClass, nil)
	if err != nil {
		return err
	}
	validationerr = validation.ValidateAWSInstanceClass(internalAWSInstanceClass)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		glog.V(2).Infof("Validation of AWSInstanceClass failled %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Manipulate finalizers
	if is.DeletionTimestamp == nil {
		c.addInstanceSetFinalizers(is)
	} else {
		c.deleteInstanceSetFinalizers(is)
	}

	selector, err := metav1.LabelSelectorAsSelector(is.Spec.Selector)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Error converting instance selector to selector: %v", err))
		return nil
	}

	// list all instances to include the instances that don't match the rs`s selector
	// anymore but has the stale controller ref.
	// TODO: Do the List and Filter in a single pass, or use an index.
	filteredInstances, err := c.instanceLister.List(labels.Everything())
	if err != nil {
		return err
	}

	// NOTE: filteredInstances are pointing to objects from cache - if you need to
	// modify them, you need to copy it first.
	filteredInstances, err = c.claimInstances(is, selector, filteredInstances)
	if err != nil {
		return err
	}

	isNeedsSync := c.expectations.SatisfiedExpectations(key)

	glog.V(4).Infof("2 Filtered instances length: %v , InstanceSetNeedsSync: %v",len(filteredInstances), isNeedsSync) 

	var manageReplicasErr error
	if isNeedsSync && is.DeletionTimestamp == nil {
		manageReplicasErr = c.manageReplicas(filteredInstances, is)
	}
	//glog.V(2).Infof("Print manageReplicasErr: %v ",manageReplicasErr) //Remove	

	is = is.DeepCopy()
	newStatus := calculateInstanceSetStatus(is, filteredInstances, manageReplicasErr)

	// Always updates status as instances come up or die.
	updatedIS, err := updateInstanceSetStatus(c.nodeClient, is, newStatus)
	if err != nil {
		// Multiple things could lead to this update failing. Requeuing the instance set ensures
		// Returning an error causes a requeue without forcing a hotloop
		glog.V(2).Infof("update instance failed with : %v ",err) //Remove
		return err
	}

	// Resync the ReplicaSet after MinReadySeconds as a last line of defense to guard against clock-skew.
	if manageReplicasErr == nil && updatedIS.Spec.MinReadySeconds > 0 &&
		updatedIS.Status.ReadyReplicas == updatedIS.Spec.Replicas &&
		updatedIS.Status.AvailableReplicas != updatedIS.Spec.Replicas {
		c.enqueueInstanceSetAfter(updatedIS, time.Duration(updatedIS.Spec.MinReadySeconds)*time.Second)
	}
	return manageReplicasErr
}

func (c *controller) claimInstances(is *v1alpha1.InstanceSet, selector labels.Selector, filteredInstances []*v1alpha1.Instance) ([]*v1alpha1.Instance, error) {
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing Instances (see #42639).
	canAdoptFunc := RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := c.nodeClient.InstanceSets().Get(is.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != is.UID {
			return nil, fmt.Errorf("original %v/%v is gone: got uid %v, wanted %v", is.Namespace, is.Name, fresh.UID, is.UID)
		}
		return fresh, nil
	})
	cm := NewInstanceControllerRefManager(c.instanceControl, is, selector, controllerKindIS, canAdoptFunc)
	return cm.ClaimInstances(filteredInstances)
}

// slowStartBatch tries to call the provided function a total of 'count' times,
// starting slow to check for errors, then speeding up if calls succeed.
//
// It groups the calls into batches, starting with a group of initialBatchSize.
// Within each batch, it may call the function multiple times concurrently.
//
// If a whole batch succeeds, the next batch may get exponentially larger.
// If there are any failures in a batch, all remaining batches are skipped
// after waiting for the current batch to complete.
//
// It returns the number of successful calls to the function.
func slowStartBatch(count int, initialBatchSize int, fn func() error) (int, error) {
	remaining := count
	successes := 0
	for batchSize := integer.IntMin(remaining, initialBatchSize); batchSize > 0; batchSize = integer.IntMin(2*batchSize, remaining) {
		errCh := make(chan error, batchSize)
		var wg sync.WaitGroup
		wg.Add(batchSize)
		for i := 0; i < batchSize; i++ {
			go func() {
				defer wg.Done()
				if err := fn(); err != nil {
					errCh <- err
				}
			}()
		}
		wg.Wait()
		curSuccesses := batchSize - len(errCh)
		successes += curSuccesses
		if len(errCh) > 0 {
			return successes, <-errCh
		}
		remaining -= batchSize
	}
	return successes, nil
}

func getInstancesToDelete(filteredInstances []*v1alpha1.Instance, diff int) []*v1alpha1.Instance {
	// No need to sort instances if we are about to delete all of them.
	// diff will always be <= len(filteredInstances), so not need to handle > case.
	if diff < len(filteredInstances) {
		// Sort the instances in the order such that not-ready < ready, unscheduled
		// < scheduled, and pending < running. This ensures that we delete instances
		// in the earlier stages whenever possible.
		sort.Sort(ActiveInstances(filteredInstances))
	}
	return filteredInstances[:diff]
}

func getInstanceKeys(instances []*v1alpha1.Instance) []string {
	instanceKeys := make([]string, 0, len(instances))
	for _, instance := range instances {
		instanceKeys = append(instanceKeys, InstanceKey(instance))
	}
	return instanceKeys
}


func (c *controller) prepareInstanceForDeletion (targetInstance *v1alpha1.Instance, is *v1alpha1.InstanceSet, wg *sync.WaitGroup, errCh *chan error) {
	defer wg.Done()

	isKey, err := KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for %v %#v: %v", is.Kind, is, err))
		return
	}
	
	// Force trigger deletion to reflect in instance status
	currentStatus := v1alpha1.CurrentStatus {
		Phase:			v1alpha1.InstanceTerminating,
		TimeoutActive:	false,
		LastUpdateTime: metav1.Now(),
	}
	c.updateInstanceStatus(targetInstance, targetInstance.Status.LastOperation, currentStatus)
	glog.V(2).Info("Delete instance from instanceset:", targetInstance .Name)

	if err := c.instanceControl.DeleteInstance(targetInstance.Name, is); err != nil {
		// Decrement the expected number of deletes because the informer won't observe this deletion
		instanceKey := InstanceKey(targetInstance)
		glog.V(2).Infof("Failed to delete %v, decrementing expectations for %v %s/%s", instanceKey, is.Kind, is.Namespace, is.Name)
		c.expectations.DeletionObserved(isKey, instanceKey)
		*errCh <- err
	}
}

func (c *controller) terminateInstances (inactiveInstances []*v1alpha1.Instance, is *v1alpha1.InstanceSet) error {
	
	var wg sync.WaitGroup
	numOfInactiveInstances := len(inactiveInstances)
	errCh := make(chan error, numOfInactiveInstances)
	wg.Add(numOfInactiveInstances)

	for _, instance := range inactiveInstances {
		go c.prepareInstanceForDeletion(instance, is, &wg, &errCh)
	}
	wg.Wait()

	select {
		case err := <-errCh:
			// all errors have been reported before and they're likely to be the same, so we'll only return the first one we hit.
			if err != nil {
				return err
			}
		default:
	}

	return nil
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addInstanceSetFinalizers (instanceSet *v1alpha1.InstanceSet) {
	clone := instanceSet.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateInstanceSetFinalizers(clone, finalizers.List())
	}
}

func (c *controller) deleteInstanceSetFinalizers (instanceSet *v1alpha1.InstanceSet) {
	clone := instanceSet.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateInstanceSetFinalizers(clone, finalizers.List())
	}
}

func (c *controller) updateInstanceSetFinalizers(instanceSet *v1alpha1.InstanceSet, finalizers []string) {
	// Get the latest version of the instanceSet so that we can avoid conflicts
	instanceSet, err := c.nodeClient.InstanceSets().Get(instanceSet.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := instanceSet.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.nodeClient.InstanceSets().Update(clone)
	if err != nil {
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying")
		c.updateInstanceSetFinalizers(instanceSet, finalizers)
	} 
}