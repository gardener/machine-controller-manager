/*
Copyright 2015 The Kubernetes Authors.

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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/deployment_controller.go

Modifications Copyright 2017 The Gardener Authors.
*/

// Package deployment contains all the logic for handling Kubernetes Deployments.
// It implements a set of strategies (rolling, recreate) for deploying an application,
// the means to rollback to previous versions, proportional scaling for mitigating
// risk, cleanup policy, and other useful features of Deployments.
package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"github.com/gardener/node-controller-manager/pkg/apis/node/v1alpha1"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("InstanceDeployment")
var GroupVersionKind = "node.sapcloud.io/v1alpha1"

func (dc *controller) addInstanceDeployment(obj interface{}) {
	d := obj.(*v1alpha1.InstanceDeployment)
	glog.V(2).Infof("Adding instance deployment %s", d.Name)
	dc.enqueueInstanceDeployment(d)
}

func (dc *controller) updateInstanceDeployment(old, cur interface{}) {
	oldD := old.(*v1alpha1.InstanceDeployment)
	curD := cur.(*v1alpha1.InstanceDeployment)
	glog.V(2).Infof("Updating instance deployment %s", oldD.Name)
	dc.enqueueInstanceDeployment(curD)
}

func (dc *controller) deleteInstanceDeployment(obj interface{}) {
	d, ok := obj.(*v1alpha1.InstanceDeployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			//utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		d, ok = tombstone.Obj.(*v1alpha1.InstanceDeployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a Instance Deployment %#v", obj))
			return
		}
	}
	glog.V(2).Infof("Deleting instance deployment %s", d.Name)
	dc.enqueueInstanceDeployment(d)
}

// addInstanceSet enqueues the deployment that manages a InstanceSet when the InstanceSet is created.
func (dc *controller) addInstanceSetToDeployment(obj interface{}) {
	is := obj.(*v1alpha1.InstanceSet)

	if is.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		dc.deleteInstanceSetToDeployment(is)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(is); controllerRef != nil {
		d := dc.resolveDeploymentControllerRef(is.Namespace, controllerRef)
		if d == nil {
			return
		}
		glog.V(4).Infof("InstanceSet %s added.", is.Name)
		dc.enqueueInstanceDeployment(d)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching Deployments and sync
	// them to see if anyone wants to adopt it.
	ds := dc.getInstanceDeploymentsForInstanceSet(is)
	if len(ds) == 0 {
		return
	}
	glog.V(4).Infof("Orphan InstanceSet %s added.", is.Name)
	for _, d := range ds {
		dc.enqueueInstanceDeployment(d)
	}
}

// getDeploymentsForInstanceSet returns a list of Deployments that potentially
// match a InstanceSet.
func (dc *controller) getInstanceDeploymentsForInstanceSet(is *v1alpha1.InstanceSet) []*v1alpha1.InstanceDeployment {
	deployments, err := dc.GetInstanceDeploymentsForInstanceSet(is)
	if err != nil || len(deployments) == 0 {
		return nil
	}
	// Because all InstanceSet's belonging to a deployment should have a unique label key,
	// there should never be more than one deployment returned by the above method.
	// If that happens we should probably dynamically repair the situation by ultimately
	// trying to clean up one of the controllers, for now we just return the older one
	if len(deployments) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		glog.V(4).Infof("user error! more than one deployment is selecting instance set %s with labels: %#v, returning %s",
			is.Name, is.Labels, deployments[0].Name)
	}
	return deployments
}

// updateInstanceSet figures out what deployment(s) manage a InstanceSet when the InstanceSet
// is updated and wake them up. If the anything of the InstanceSets have changed, we need to
// awaken both the old and new deployments. old and cur must be *extensions.InstanceSet
// types.
func (dc *controller) updateInstanceSetToDeployment(old, cur interface{}) {
	curIS := cur.(*v1alpha1.InstanceSet)
	oldIS := old.(*v1alpha1.InstanceSet)
	if curIS.ResourceVersion == oldIS.ResourceVersion {
		// Periodic resync will send update events for all known instance sets.
		// Two different versions of the same instance set will always have different RVs.
		return
	}

	curControllerRef := metav1.GetControllerOf(curIS)
	oldControllerRef := metav1.GetControllerOf(oldIS)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if d := dc.resolveDeploymentControllerRef(oldIS.Namespace, oldControllerRef); d != nil {
			dc.enqueueInstanceDeployment(d)
		}
	}

	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		d := dc.resolveDeploymentControllerRef(curIS.Namespace, curControllerRef)
		if d == nil {
			return
		}
		glog.V(4).Infof("InstanceSet %s updated.", curIS.Name)
		dc.enqueueInstanceDeployment(d)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	labelChanged := !reflect.DeepEqual(curIS.Labels, oldIS.Labels)
	if labelChanged || controllerRefChanged {
		ds := dc.getInstanceDeploymentsForInstanceSet(curIS)
		if len(ds) == 0 {
			return
		}
		glog.V(4).Infof("Orphan InstanceSet %s updated.", curIS.Name)
		for _, d := range ds {
			dc.enqueueInstanceDeployment(d)
		}
	}
}

// deleteInstanceSet enqueues the deployment that manages a InstanceSet when
// the InstanceSet is deleted. obj could be an *extensions.InstanceSet, or
// a DeletionFinalStateUnknown marker item.
func (dc *controller) deleteInstanceSetToDeployment(obj interface{}) {
	is, ok := obj.(*v1alpha1.InstanceSet)

	// When a delete is dropped, the relist will notice a Instance in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the InstanceSet
	// changed labels the new deployment will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		is, ok = tombstone.Obj.(*v1alpha1.InstanceSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a InstanceSet %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(is)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	d := dc.resolveDeploymentControllerRef(is.Namespace, controllerRef)
	if d == nil {
		return
	}
	glog.V(4).Infof("InstanceSet %s deleted.", is.Name)
	dc.enqueueInstanceDeployment(d)
}

// deleteInstance will enqueue a Recreate Deployment once all of its Instances have stopped running.
func (dc *controller) deleteInstanceToInstanceDeployment(obj interface{}) {
	instance, ok := obj.(*v1alpha1.Instance)

	// When a delete is dropped, the relist will notice a Instance in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the Instance
	// changed labels the new deployment will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		instance, ok = tombstone.Obj.(*v1alpha1.Instance)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a instance %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Instance %s deleted.", instance.Name)
	if d := dc.getInstanceDeploymentForInstance(instance); d != nil && d.Spec.Strategy.Type == v1alpha1.RecreateInstanceDeploymentStrategyType {
		// Sync if this Deployment now has no more Instances.
		isList, err := ListInstanceSets(d, IsListFromClient(dc.nodeClient))
		if err != nil {
			return
		}
		instanceMap, err := dc.getInstanceMapForInstanceDeployment(d, isList)
		if err != nil {
			return
		}
		numInstances := 0
		for _, instanceList := range instanceMap {
			numInstances += len(instanceList.Items)
		}
		if numInstances == 0 {
			dc.enqueueInstanceDeployment(d)
		}
	}
}

func (dc *controller) enqueueInstanceDeployment(deployment *v1alpha1.InstanceDeployment) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", deployment, err))
		return
	}

	dc.instanceDeploymentQueue.Add(key)
}

func (dc *controller) enqueueRateLimited(deployment *v1alpha1.InstanceDeployment) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", deployment, err))
		return
	}

	dc.instanceDeploymentQueue.AddRateLimited(key)
}

// enqueueAfter will enqueue a deployment after the provided amount of time.
func (dc *controller) enqueueAfter(deployment *v1alpha1.InstanceDeployment, after time.Duration) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", deployment, err))
		return
	}

	dc.instanceDeploymentQueue.AddAfter(key, after)
}

// getDeploymentForInstance returns the deployment managing the given Instance.
func (dc *controller) getInstanceDeploymentForInstance(instance *v1alpha1.Instance) *v1alpha1.InstanceDeployment {
	// Find the owning instance set
	var is *v1alpha1.InstanceSet
	var err error
	controllerRef := metav1.GetControllerOf(instance)
	if controllerRef == nil {
		// No controller owns this Instance.
		return nil
	}
	if controllerRef.Kind != "InstanceDeployment" { //TODO: Remove hardcoded string
 		// Not a Instance owned by a instance set.
		return nil
	}
	is, err = dc.nodeClient.InstanceSets().Get(controllerRef.Name, metav1.GetOptions{})
	if err != nil || is.UID != controllerRef.UID {
		glog.V(4).Infof("Cannot get instanceset %q for instance %q: %v", controllerRef.Name, instance.Name, err)
		return nil
	}

	// Now find the Deployment that owns that InstanceSet.
	controllerRef = metav1.GetControllerOf(is)
	if controllerRef == nil {
		return nil
	}
	return dc.resolveDeploymentControllerRef(is.Namespace, controllerRef)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (dc *controller) resolveDeploymentControllerRef(namespace string, controllerRef *metav1.OwnerReference) *v1alpha1.InstanceDeployment {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}
	d, err := dc.nodeClient.InstanceDeployments().Get(controllerRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil
	}
	if d.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return d
}

func (dc *controller) handleErr(err error, key interface{}) {
	if err == nil {
		dc.instanceDeploymentQueue.Forget(key)
		return
	}

	if dc.instanceDeploymentQueue.NumRequeues(key) < maxRetries {
		glog.V(2).Infof("Error syncing deployment %v: %v", key, err)
		dc.instanceDeploymentQueue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(2).Infof("Dropping deployment %q out of the queue: %v", key, err)
	dc.instanceDeploymentQueue.Forget(key)
}

// getInstanceSetsForDeployment uses ControllerRefManager to reconcile
// ControllerRef by adopting and orphaning.
// It returns the list of InstanceSets that this Deployment should manage.
func (dc *controller) getInstanceSetsForInstanceDeployment(d *v1alpha1.InstanceDeployment) ([]*v1alpha1.InstanceSet, error) {
	// List all InstanceSets to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimInstanceSets().
	isList, err := dc.instanceSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	deploymentSelector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("instance deployment %s has invalid label selector: %v", d.Name, err)
	}
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing InstanceSets (see #42639).
	canAdoptFunc := RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := dc.nodeClient.InstanceDeployments().Get(d.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != d.UID {
			return nil, fmt.Errorf("original Instance Deployment %v is gone: got uid %v, wanted %v", d.Name, fresh.UID, d.UID)
		}
		return fresh, nil
	})
	cm := NewInstanceSetControllerRefManager(dc.instanceSetControl, d, deploymentSelector, controllerKind, canAdoptFunc)
	ISes, err := cm.ClaimInstanceSets(isList)
	return ISes, err
}

// getInstanceMapForDeployment returns the Instances managed by a Deployment.
//
// It returns a map from InstanceSet UID to a list of Instances controlled by that RS,
// according to the Instance's ControllerRef.
func (dc *controller) getInstanceMapForInstanceDeployment(d *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet) (map[types.UID]*v1alpha1.InstanceList, error) {
	// Get all Instances that potentially belong to this Deployment.
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, err
	}
	instances, err := dc.instanceLister.List(selector)
	if err != nil {
		return nil, err
	}
	// Group Instances by their controller (if it's in rsList).
	instanceMap := make(map[types.UID]*v1alpha1.InstanceList, len(isList))
	for _, is := range isList {
		instanceMap[is.UID] = &v1alpha1.InstanceList{}
	}
	for _, instance := range instances {
		// Do not ignore inactive Instances because Recreate Deployments need to verify that no
		// Instances from older versions are running before spinning up new Instances.
		controllerRef := metav1.GetControllerOf(instance)
		if controllerRef == nil {
			continue
		}
		// Only append if we care about this UID.
		if instanceList, ok := instanceMap[controllerRef.UID]; ok {
			instanceList.Items = append(instanceList.Items, *instance)
		}
	}
	return instanceMap, nil
}

// syncDeployment will sync the deployment with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (dc *controller) syncInstanceDeployment(key string) error {
	startTime := time.Now()
	glog.V(4).Infof("Started syncing deployment %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing deployment %q (%v)", key, time.Since(startTime))
	}()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	deployment, err := dc.nodeClient.InstanceDeployments().Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		glog.V(2).Infof("Deployment %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	d := deployment.DeepCopy()

	everything := metav1.LabelSelector{}
	if reflect.DeepEqual(d.Spec.Selector, &everything) {
		dc.recorder.Eventf(d, v1.EventTypeWarning, "SelectingAll", "This deployment is selecting all instances. A non-empty selector is required.")
		if d.Status.ObservedGeneration < d.Generation {
			d.Status.ObservedGeneration = d.Generation
			dc.nodeClient.InstanceDeployments().Update(d)
		}
		return nil
	}

	// List InstanceSets owned by this Deployment, while reconciling ControllerRef
	// through adoption/orphaning.
	isList, err := dc.getInstanceSetsForInstanceDeployment(d)
	if err != nil {
		return err
	}
	// List all Instances owned by this Deployment, grouped by their InstanceSet.
	// Current uses of the InstanceMap are:
	//
	// * check if a Instance is labeled correctly with the Instance-template-hash label.
	// * check that no old Instances are running in the middle of Recreate Deployments.
	instanceMap, err := dc.getInstanceMapForInstanceDeployment(d, isList)
	if err != nil {
		return err
	}

	if d.DeletionTimestamp != nil {
		return dc.syncStatusOnly(d, isList, instanceMap)
	}

	// Update deployment conditions with an Unknown condition when pausing/resuming
	// a deployment. In this way, we can be sure that we won't timeout when a user
	// resumes a Deployment with a set progressDeadlineSeconds.
	if err = dc.checkPausedConditions(d); err != nil {
		return err
	}

	if d.Spec.Paused {
		return dc.sync(d, isList, instanceMap)
	}

	// rollback is not re-entrant in case the underlying instance sets are updated with a new
	// revision so we should ensure that we won't proceed to update instance sets until we
	// make sure that the deployment has cleaned up its rollback spec in subsequent enqueues.
	if d.Spec.RollbackTo != nil {
		return dc.rollback(d, isList, instanceMap)
	}

	scalingEvent, err := dc.isScalingEvent(d, isList, instanceMap)

	if err != nil {
		return err
	}
	if scalingEvent {
		return dc.sync(d, isList, instanceMap)
	}

	switch d.Spec.Strategy.Type {
	case v1alpha1.RecreateInstanceDeploymentStrategyType:
		return dc.rolloutRecreate(d, isList, instanceMap)
	case v1alpha1.RollingUpdateInstanceDeploymentStrategyType:
		return dc.rolloutRolling(d, isList, instanceMap)
	}
	return fmt.Errorf("unexpected deployment strategy type: %s", d.Spec.Strategy.Type)
}
