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

Modifications Contributors to the Gardener project
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/constants"
	"github.com/gardener/machine-controller-manager/pkg/util/nodeops"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machinecodes/codes"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/validation"
	"github.com/gardener/machine-controller-manager/pkg/util/annotations"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

// triggerDeletionData is used to store the data related to the machines for which deletion
// should be triggered by MCM and the time when scaler decided to scale-down those machines.
// This data is stored in the TriggerDeletionByMCM annotation on the MachineDeployment.
type triggerDeletionData struct {
	markedMachines                        []*v1alpha1.Machine
	markedMachineDeletionTimes            []string
	triggerDeletionAnnotationValue        string
	triggerDeletionAnnotationValueChanged bool
}

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = v1alpha1.SchemeGroupVersion.WithKind("MachineDeployment")

// GroupVersionKind is the version kind used to identify objects managed by machine-controller-manager
var GroupVersionKind = "machine.sapcloud.io/v1alpha1"

func (c *controller) addMachineDeployment(obj any) {
	d := obj.(*v1alpha1.MachineDeployment)
	klog.V(4).Infof("Adding machine deployment %s", d.Name)
	c.enqueueMachineDeployment(d)
}

func (c *controller) updateMachineDeployment(old, cur any) {
	oldD := old.(*v1alpha1.MachineDeployment)
	curD := cur.(*v1alpha1.MachineDeployment)
	klog.V(4).Infof("Updating machine deployment %s", oldD.Name)
	c.enqueueMachineDeployment(curD)
}

func (c *controller) deleteMachineDeployment(obj any) {
	d, ok := obj.(*v1alpha1.MachineDeployment)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			//utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		d, ok = tombstone.Obj.(*v1alpha1.MachineDeployment)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a MachineDeployment %#v", obj))
			return
		}
	}
	klog.V(4).Infof("Deleting machine deployment %s", d.Name)
	c.enqueueMachineDeployment(d)
}

// addMachineSet enqueues the deployment that manages a MachineSet when the MachineSet is created.
func (c *controller) addMachineSetToDeployment(obj any) {
	is := obj.(*v1alpha1.MachineSet)

	if is.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		c.deleteMachineSetToDeployment(is)
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(is); controllerRef != nil {
		d := c.resolveDeploymentControllerRef(is.Namespace, controllerRef)
		if d == nil {
			return
		}
		klog.V(4).Infof("MachineSet %s added.", is.Name)
		c.enqueueMachineDeployment(d)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching Deployments and sync
	// them to see if anyone wants to adopt it.
	ds := c.getMachineDeploymentsForMachineSet(is)
	if len(ds) == 0 {
		return
	}
	klog.V(4).Infof("Orphan MachineSet %s added.", is.Name)
	for _, d := range ds {
		c.enqueueMachineDeployment(d)
	}
}

// getDeploymentsForMachineSet returns a list of Deployments that potentially
// match a MachineSet.
func (c *controller) getMachineDeploymentsForMachineSet(machineSet *v1alpha1.MachineSet) []*v1alpha1.MachineDeployment {
	deployments, err := c.GetMachineDeploymentsForMachineSet(machineSet)
	if err != nil || len(deployments) == 0 {
		return nil
	}
	// Because all MachineSet's belonging to a deployment should have a unique label key,
	// there should never be more than one deployment returned by the above method.
	// If that happens we should probably dynamically repair the situation by ultimately
	// trying to clean up one of the controllers, for now we just return the older one
	if len(deployments) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		klog.Errorf("user error! more than one deployment is selecting machine set %s with labels: %#v, returning %s",
			machineSet.Name, machineSet.Labels, deployments[0].Name)
	}
	return deployments
}

// updateMachineSet figures out what deployment(s) manage a MachineSet when the MachineSet
// is updated and wake them up. If the anything of the MachineSets have changed, we need to
// awaken both the old and new deployments. old and cur must be *extensions.MachineSet
// types.
func (c *controller) updateMachineSetToDeployment(old, cur any) {
	curMachineSet := cur.(*v1alpha1.MachineSet)
	oldMachineSet := old.(*v1alpha1.MachineSet)
	if curMachineSet.ResourceVersion == oldMachineSet.ResourceVersion {
		// Periodic resync will send update events for all known machine sets.
		// Two different versions of the same machine set will always have different RVs.
		return
	}

	curControllerRef := metav1.GetControllerOf(curMachineSet)
	oldControllerRef := metav1.GetControllerOf(oldMachineSet)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if d := c.resolveDeploymentControllerRef(oldMachineSet.Namespace, oldControllerRef); d != nil {
			c.enqueueMachineDeployment(d)
		}
	}

	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		d := c.resolveDeploymentControllerRef(curMachineSet.Namespace, curControllerRef)
		if d == nil {
			return
		}
		klog.V(4).Infof("MachineSet %s updated.", curMachineSet.Name)
		c.enqueueMachineDeployment(d)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	labelChanged := !reflect.DeepEqual(curMachineSet.Labels, oldMachineSet.Labels)
	if labelChanged || controllerRefChanged {
		ds := c.getMachineDeploymentsForMachineSet(curMachineSet)
		if len(ds) == 0 {
			return
		}
		klog.V(4).Infof("Orphan MachineSet %s updated.", curMachineSet.Name)
		for _, d := range ds {
			c.enqueueMachineDeployment(d)
		}
	}
}

// deleteMachineSet enqueues the deployment that manages a MachineSet when
// the MachineSet is deleted. obj could be an *v1alpha1.MachineSet, or
// a DeletionFinalStateUnknown marker item.
func (c *controller) deleteMachineSetToDeployment(obj any) {
	machineSet, ok := obj.(*v1alpha1.MachineSet)

	// When a delete is dropped, the relist will notice a Machine in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the MachineSet
	// changed labels the new deployment will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		machineSet, ok = tombstone.Obj.(*v1alpha1.MachineSet)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a MachineSet %#v", obj))
			return
		}
	}

	controllerRef := metav1.GetControllerOf(machineSet)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	d := c.resolveDeploymentControllerRef(machineSet.Namespace, controllerRef)
	if d == nil {
		return
	}
	klog.V(4).Infof("MachineSet %s deleted.", machineSet.Name)
	c.enqueueMachineDeployment(d)
}

// updateMachineToMachineDeployment will enqueue the machine deployment if the machine InPlaceUpdate node condition changes to UpdateSuccessful.
func (c *controller) updateMachineToMachineDeployment(old, cur any) {
	oldMachine, ok := old.(*v1alpha1.Machine)
	if !ok {
		return
	}

	curMachine, ok := cur.(*v1alpha1.Machine)
	if !ok {
		return
	}

	oldMachineCondition := getMachineCondition(oldMachine, v1alpha1.NodeInPlaceUpdate)
	currMachineCondition := getMachineCondition(curMachine, v1alpha1.NodeInPlaceUpdate)

	oldMachineConditionReasonUpdateSuccessful := oldMachineCondition != nil && oldMachineCondition.Reason == v1alpha1.UpdateSuccessful
	currMachineConditionReasonUpdateSuccessful := currMachineCondition != nil && currMachineCondition.Reason == v1alpha1.UpdateSuccessful

	if !oldMachineConditionReasonUpdateSuccessful && currMachineConditionReasonUpdateSuccessful {
		d := c.getMachineDeploymentForMachine(curMachine)
		if d != nil {
			c.enqueueMachineDeployment(d)
		}
	}
}

// deleteMachine will enqueue a Recreate Deployment once all of its Machines have stopped running.
func (c *controller) deleteMachineToMachineDeployment(obj any) {
	ctx := context.Background()
	machine, ok := obj.(*v1alpha1.Machine)

	// When a delete is dropped, the relist will notice a Machine in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the Machine
	// changed labels the new deployment will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		machine, ok = tombstone.Obj.(*v1alpha1.Machine)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a machine %#v", obj))
			return
		}
	}
	klog.V(4).Infof("Machine %s deleted.", machine.Name)
	if d := c.getMachineDeploymentForMachine(machine); d != nil && d.Spec.Strategy.Type == v1alpha1.RecreateMachineDeploymentStrategyType {
		// Sync if this Deployment now has no more Machines.
		machineSets, err := ListMachineSets(d, IsListFromClient(ctx, c.controlMachineClient))
		if err != nil {
			return
		}
		machineMap, err := c.getMachineMapForMachineDeployment(d, machineSets)
		if err != nil {
			return
		}
		numMachines := 0
		for _, machineList := range machineMap {
			numMachines += len(machineList.Items)
		}
		if numMachines == 0 {
			c.enqueueMachineDeployment(d)
		}
	}
}

func (c *controller) enqueueMachineDeployment(deployment *v1alpha1.MachineDeployment) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", deployment, err))
		return
	}

	c.machineDeploymentQueue.Add(key)
}

func (c *controller) enqueueRateLimited(deployment *v1alpha1.MachineDeployment) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", deployment, err))
		return
	}

	c.machineDeploymentQueue.AddRateLimited(key)
}

// enqueueMachineDeploymentAfter will enqueue a deployment after the provided amount of time.
func (c *controller) enqueueMachineDeploymentAfter(deployment *v1alpha1.MachineDeployment, after time.Duration) {
	key, err := KeyFunc(deployment)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", deployment, err))
		return
	}

	c.machineDeploymentQueue.AddAfter(key, after)
}

// getDeploymentForMachine returns the deployment managing the given Machine.
func (c *controller) getMachineDeploymentForMachine(machine *v1alpha1.Machine) *v1alpha1.MachineDeployment {
	// Find the owning machine set
	var is *v1alpha1.MachineSet
	var err error
	controllerRef := metav1.GetControllerOf(machine)
	if controllerRef == nil {
		// No controller owns this Machine.
		return nil
	}
	if controllerRef.Kind != "MachineSet" { //TODO: Remove hardcoded string
		// Not a Machine owned by a machine set.
		return nil
	}

	is, err = c.machineSetLister.MachineSets(machine.Namespace).Get(controllerRef.Name)
	if err != nil || is.UID != controllerRef.UID {
		klog.V(4).Infof("Cannot get machineset %q for machine %q: %v", controllerRef.Name, machine.Name, err)
		return nil
	}

	// Now find the Deployment that owns that MachineSet.
	controllerRef = metav1.GetControllerOf(is)
	if controllerRef == nil {
		return nil
	}
	return c.resolveDeploymentControllerRef(is.Namespace, controllerRef)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (c *controller) resolveDeploymentControllerRef(namespace string, controllerRef *metav1.OwnerReference) *v1alpha1.MachineDeployment {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != controllerKind.Kind {
		return nil
	}
	d, err := c.controlMachineClient.MachineDeployments(namespace).Get(context.TODO(), controllerRef.Name, metav1.GetOptions{})
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

// getMachineSetsForDeployment uses ControllerRefManager to reconcile
// ControllerRef by adopting and orphaning.
// It returns the list of MachineSets that this Deployment should manage.
func (c *controller) getMachineSetsForMachineDeployment(ctx context.Context, d *v1alpha1.MachineDeployment) ([]*v1alpha1.MachineSet, error) {
	// List all MachineSets to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimMachineSets().
	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	deploymentSelector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("machine deployment %s has invalid label selector: %v", d.Name, err)
	}
	// If any adoptions are attempted, we should first recheck for deletion with
	// an uncached quorum read sometime after listing MachineSets (see #42639).
	canAdoptFunc := RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := c.controlMachineClient.MachineDeployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != d.UID {
			return nil, fmt.Errorf("original Machine Deployment %v is gone: got uid %v, wanted %v", d.Name, fresh.UID, d.UID)
		}
		return fresh, nil
	})
	cm := NewMachineSetControllerRefManager(c.machineSetControl, d, deploymentSelector, controllerKind, canAdoptFunc)
	ISes, err := cm.ClaimMachineSets(ctx, machineSets)
	return ISes, err
}

// getMachineMapForDeployment returns the Machines managed by a Deployment.
//
// It returns a map from MachineSet UID to a list of Machines controlled by that RS,
// according to the Machine's ControllerRef.
func (c *controller) getMachineMapForMachineDeployment(d *v1alpha1.MachineDeployment, machineSets []*v1alpha1.MachineSet) (map[types.UID]*v1alpha1.MachineList, error) {
	// Get all Machines that potentially belong to this Deployment.
	selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	if err != nil {
		return nil, err
	}
	machines, err := c.machineLister.List(selector)
	if err != nil {
		return nil, err
	}
	// Group Machines by their controller (if it's in rsList).
	machineMap := make(map[types.UID]*v1alpha1.MachineList, len(machineSets))
	for _, is := range machineSets {
		machineMap[is.UID] = &v1alpha1.MachineList{}
	}
	for _, machine := range machines {
		// Do not ignore inactive Machines because Recreate Deployments need to verify that no
		// Machines from older versions are running before spinning up new Machines.
		controllerRef := metav1.GetControllerOf(machine)
		if controllerRef == nil {
			continue
		}
		// Only append if we care about this UID.
		if machineList, ok := machineMap[controllerRef.UID]; ok {
			machineList.Items = append(machineList.Items, *machine)
		}
	}
	return machineMap, nil
}

// reconcileClusterMachineDeployment will sync the deployment with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (c *controller) reconcileClusterMachineDeployment(key string) error {
	ctx := context.Background()
	startTime := time.Now()
	klog.V(4).Infof("Started syncing machine deployment %q (%v)", key, startTime)
	defer func() {
		klog.V(4).Infof("Finished syncing machine deployment %q (%v)", key, time.Since(startTime))
	}()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	deployment, err := c.controlMachineClient.MachineDeployments(c.namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		klog.V(4).Infof("Deployment %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	klog.V(3).Infof("Processing the machinedeployment %q (with replicas %d)", deployment.Name, deployment.Spec.Replicas)

	// If MachineDeployment is frozen and no deletion timestamp, don't process it
	if deployment.Labels["freeze"] == "True" && deployment.DeletionTimestamp == nil {
		klog.V(3).Infof("MachineDeployment %q is frozen. However, it will still be processed if it there is an scale down event.", deployment.Name)
	}

	// Validate MachineDeployment
	internalMachineDeployment := &machine.MachineDeployment{}

	err = v1alpha1.Convert_v1alpha1_MachineDeployment_To_machine_MachineDeployment(deployment, internalMachineDeployment, nil)
	if err != nil {
		return err
	}

	validationerr := validation.ValidateMachineDeployment(internalMachineDeployment)
	if validationerr.ToAggregate() != nil && len(validationerr.ToAggregate().Errors()) > 0 {
		klog.Errorf("Validation of MachineDeployment failed %s", validationerr.ToAggregate().Error())
		return nil
	}

	// Resync the MachineDeployment after 10 minutes to avoid missing out on missed out events
	defer c.enqueueMachineDeploymentAfter(deployment, 10*time.Minute)

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	d := deployment.DeepCopy()

	// Manipulate finalizers
	if d.DeletionTimestamp == nil {
		c.addMachineDeploymentFinalizers(ctx, d)
	}

	everything := metav1.LabelSelector{}
	if reflect.DeepEqual(d.Spec.Selector, &everything) {
		c.recorder.Eventf(d, v1.EventTypeWarning, "SelectingAll", "This deployment is selecting all machines. A non-empty selector is required.")
		if d.Status.ObservedGeneration < d.Generation {
			d.Status.ObservedGeneration = d.Generation
			if _, err := c.controlMachineClient.MachineDeployments(d.Namespace).UpdateStatus(ctx, d, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update status for machine deployment %s: %w", deployment.Name, err)

			}
		}
		return nil
	}

	// List MachineSets owned by this Deployment, while reconciling ControllerRef
	// through adoption/orphaning.
	machineSets, err := c.getMachineSetsForMachineDeployment(ctx, d)
	if err != nil {
		return err
	}
	// List all Machines owned by this Deployment, grouped by their MachineSet.
	// Current uses of the MachineMap are:
	//
	// * check if a Machine is labeled correctly with the Machine-template-hash label.
	// * check that no old Machines are running in the middle of Recreate Deployments.
	machineMap, err := c.getMachineMapForMachineDeployment(d, machineSets)
	if err != nil {
		return err
	}

	if d.DeletionTimestamp != nil {
		if finalizers := sets.NewString(d.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
			return nil
		}
		if len(machineSets) == 0 {
			c.deleteMachineDeploymentFinalizers(ctx, d)
			return nil
		}
		klog.V(4).Infof("Deleting all child MachineSets as MachineDeployment %s has set deletionTimestamp", d.Name)
		c.terminateMachineSets(ctx, machineSets, d)
		return c.syncStatusOnly(ctx, d, machineSets, machineMap)
	}

	// Update deployment conditions with an Unknown condition when pausing/resuming
	// a deployment. In this way, we can be sure that we won't timeout when a user
	// resumes a Deployment with a set progressDeadlineSeconds.
	if err = c.checkPausedConditions(ctx, d); err != nil {
		return err
	}

	// Temporary code for backward compatibility, can be removed in later release
	d, err = c.adjustingMachineDeploymentDeletionAnnotations(ctx, d)
	if err != nil {
		return err
	}

	err = c.updateMachineAndMachineDeploymentDeletionAnnotations(ctx, d)
	if err != nil {
		return err
	}

	if d.Spec.Paused {
		klog.V(3).Infof("Scaling detected for machineDeployment %s which is paused", d.Name)
		return c.sync(ctx, d, machineSets, machineMap)
	}

	// rollback is not re-entrant in case the underlying machine sets are updated with a new
	// revision so we should ensure that we won't proceed to update machine sets until we
	// make sure that the deployment has cleaned up its rollback spec in subsequent enqueues.
	if d.Spec.RollbackTo != nil {
		return c.rollback(ctx, d, machineSets, machineMap)
	}

	if adjusted, err := c.checkAndAdjustMachineEffectiveCreationTimeout(ctx, d, machineMap); adjusted {
		return nil
	} else if err != nil {
		klog.Warningf("could not check and adjust %q: %v", v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout, err)
	}

	scalingEvent, err := c.isScalingEvent(ctx, d, machineSets, machineMap)

	if err != nil {
		return err
	}
	if scalingEvent {
		klog.V(3).Infof("Scaling detected for machineDeployment %s", d.Name)
		return c.sync(ctx, d, machineSets, machineMap)
	}

	switch d.Spec.Strategy.Type {
	case v1alpha1.RecreateMachineDeploymentStrategyType:
		return c.rolloutRecreate(ctx, d, machineSets, machineMap)
	case v1alpha1.RollingUpdateMachineDeploymentStrategyType:
		return c.rolloutRolling(ctx, d, machineSets, machineMap)
	case v1alpha1.InPlaceUpdateMachineDeploymentStrategyType:
		return c.rolloutInPlace(ctx, d, machineSets, machineMap)
	}

	return fmt.Errorf("unexpected deployment strategy type: %s", d.Spec.Strategy.Type)
}

func (c *controller) terminateMachineSets(ctx context.Context, machineSets []*v1alpha1.MachineSet, _ *v1alpha1.MachineDeployment) {
	var (
		wg               sync.WaitGroup
		numOfMachinesets = len(machineSets)
	)
	wg.Add(numOfMachinesets)

	for _, machineSet := range machineSets {
		go func(machineSet *v1alpha1.MachineSet) {
			defer wg.Done()
			// Machine is already marked as 'to-be-deleted'
			if machineSet.DeletionTimestamp != nil {
				return
			}
			if err := c.controlMachineClient.MachineSets(machineSet.Namespace).Delete(ctx, machineSet.Name, metav1.DeleteOptions{}); err != nil {
				klog.Errorf("failed to delete machineset %s: %v", machineSet.Name, err)
			}
		}(machineSet)
	}
	wg.Wait()
}

/*
	SECTION
	Manipulate Finalizers
*/

func (c *controller) addMachineDeploymentFinalizers(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment) {
	clone := machineDeployment.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); !finalizers.Has(DeleteFinalizerName) {
		finalizers.Insert(DeleteFinalizerName)
		c.updateMachineDeploymentFinalizers(ctx, clone, finalizers.List())
	}
}

func (c *controller) deleteMachineDeploymentFinalizers(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment) {
	clone := machineDeployment.DeepCopy()

	if finalizers := sets.NewString(clone.Finalizers...); finalizers.Has(DeleteFinalizerName) {
		finalizers.Delete(DeleteFinalizerName)
		c.updateMachineDeploymentFinalizers(ctx, clone, finalizers.List())
	}
}

func (c *controller) updateMachineDeploymentFinalizers(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, finalizers []string) {
	// Get the latest version of the machineDeployment so that we can avoid conflicts
	machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(ctx, machineDeployment.Name, metav1.GetOptions{})
	if err != nil {
		return
	}

	clone := machineDeployment.DeepCopy()
	clone.Finalizers = finalizers
	_, err = c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		// Keep retrying until update goes through
		klog.Warning("Updated failed, retrying")
		c.updateMachineDeploymentFinalizers(ctx, machineDeployment, finalizers)
	}
}

func (c *controller) updateMachineAndMachineDeploymentDeletionAnnotations(ctx context.Context, mcd *v1alpha1.MachineDeployment) (err error) {
	tgd := c.computeMachineTriggerDeletionData(mcd)
	if tgd == nil {
		return nil
	}

	if tgd.triggerDeletionAnnotationValueChanged {
		mcdDeepCopy := mcd.DeepCopy()
		if mcdDeepCopy.Annotations == nil {
			mcdDeepCopy.Annotations = make(map[string]string)
		}
		mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM] = tgd.triggerDeletionAnnotationValue
		if mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM] == "" {
			delete(mcdDeepCopy.Annotations, machineutils.TriggerDeletionByMCM)
		}
		_, err = c.controlMachineClient.MachineDeployments(mcd.Namespace).Update(ctx, mcdDeepCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update MachineDeployment %q with #%d machine names still pending deletion, triggerDeletionAnnotValue=%q", mcd.Name, len(tgd.markedMachines), mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM])
			return
		}
		klog.V(3).Infof("Updated MachineDeployment %q with #%d machines still pending deletion, triggerDeletionAnnotValue=%q", mcd.Name, len(tgd.markedMachines), mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM])
	}

	for i, machine := range tgd.markedMachines {
		if machine.Annotations[machineutils.MachinePriority] == "1" && machine.Annotations[machineutils.MarkedForDeletionTime] != "" {
			klog.V(4).Infof("Machine %q of MachineDeployment %q already has MachinePriority=1 and MarkedForDeletionTime=%q annotation", machine.Name, mcd.Name, machine.Annotations[machineutils.MarkedForDeletionTime])
			continue
		}
		machineDeepCopy := machine.DeepCopy()
		if machineDeepCopy.Annotations == nil {
			machineDeepCopy.Annotations = make(map[string]string)
		}
		machineDeepCopy.Annotations[machineutils.MachinePriority] = "1"
		if machineDeepCopy.Annotations[machineutils.MarkedForDeletionTime] == "" {
			machineDeepCopy.Annotations[machineutils.MarkedForDeletionTime] = tgd.markedMachineDeletionTimes[i]
		}
		_, err = c.controlMachineClient.Machines(machine.Namespace).Update(ctx, machineDeepCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to set MachinePriority=1 annotation on Machine %q of MachineDeployment %q: %v", machine.Name, mcd.Name, err)
			return
		}
		klog.V(3).Infof("Machine %q of MachineDeployment %q marked with MachinePriority=1 annotation successfully", machine.Name, mcd.Name)
	}

	return
}

// computeMachineTriggerDeletionData computes the data related to machines that are triggered for deletion based on the annotation on the MachineDeployment.
func (c *controller) computeMachineTriggerDeletionData(mcd *v1alpha1.MachineDeployment) *triggerDeletionData {
	oldTriggerDeletionAnnotationList := annotations.GetMachineNamesWithDeletionTimesTriggeredForDeletion(mcd)
	newTriggerDeletionAnnotationList := make([]string, 0)
	markedMachines := make([]*v1alpha1.Machine, 0)
	markedMachineDeletionTimes := make([]string, 0)

	if len(oldTriggerDeletionAnnotationList) == 0 {
		return nil
	}
	klog.Infof("MachineDeployment %q has #%d machine(s) marked for deletion, triggerForDeletionMachineNames=%v", mcd.Name, len(oldTriggerDeletionAnnotationList), oldTriggerDeletionAnnotationList)

	for _, machineNameWithTime := range oldTriggerDeletionAnnotationList {
		parts := strings.Split(machineNameWithTime, "~")
		// We don't add the machine name with time that has invalid format into newTriggerDeletionAnnotationList to make sure that it is removed from the annotation
		// and expect scaler to put the correct format in the annotation in the next retry.
		if len(parts) != 2 {
			klog.Infof("Invalid formatting in entry %q in MachineDeployment %q annotation value. Expected format is <machineName1>~<deletionTime1>,<machineName2>~<deletionTime2>; skipping setting MachinePriority=1 annotation", machineNameWithTime, mcd.Name)
			continue
		}
		machineName, machineDeletionTime := parts[0], parts[1]
		if _, perr := time.Parse(time.RFC3339, machineDeletionTime); perr != nil {
			klog.Warningf("Invalid formatting of deletion time %q for machine %q in MachineDeployment %q annotation", machineDeletionTime, machineName, mcd.Name)
			continue
		}
		machine, gerr := c.machineLister.Machines(c.namespace).Get(machineName)
		// The machine is deleted and hence we can remove it from the annotation.
		if apierrors.IsNotFound(gerr) {
			klog.V(4).Infof("Machine %q is not found in MachineDeployment %q - skip adding to newTriggerDeletionAnnotationList", machineName, mcd.Name)
			continue
		}
		// The machine is in the process of being deleted hence we can remove it from the annotation and expect it to be deleted in the next retry.
		if machineutils.IsFailedOrTerminating(machine) {
			klog.V(4).Infof("Machine %q of MachineDeployment %q is in Failed/Terminating state; skipping adding to newTriggerDeletionAnnotationList", machineName, mcd.Name)
			continue
		}
		newTriggerDeletionAnnotationList = append(newTriggerDeletionAnnotationList, machineNameWithTime)
		markedMachines = append(markedMachines, machine)
		markedMachineDeletionTimes = append(markedMachineDeletionTimes, machineDeletionTime)
	}

	newTriggerDeletionAnnotationValue := strings.Join(newTriggerDeletionAnnotationList, ",")
	return &triggerDeletionData{
		triggerDeletionAnnotationValueChanged: newTriggerDeletionAnnotationValue != mcd.Annotations[machineutils.TriggerDeletionByMCM],
		triggerDeletionAnnotationValue:        newTriggerDeletionAnnotationValue,
		markedMachines:                        markedMachines,
		markedMachineDeletionTimes:            markedMachineDeletionTimes,
	}
}

// TODO: separate the logic of adjusting the annotation value and updating the annotation on the MachineDeployment into two functions, and add unit tests for the function that computes the new annotation value.
func (c *controller) adjustingMachineDeploymentDeletionAnnotations(ctx context.Context, mcd *v1alpha1.MachineDeployment) (*v1alpha1.MachineDeployment, error) {
	if mcd.Annotations[machineutils.TriggerDeletionByMCM] == "" {
		return mcd, nil
	}

	mcdDeepCopy := mcd.DeepCopy()
	if mcdDeepCopy.Annotations[machineutils.LastDeploymentReplicaChangeByScalerTime] == "" {
		mcdDeepCopy.Annotations[machineutils.LastDeploymentReplicaChangeByScalerTime] = time.Now().Format(time.RFC3339)
	}
	timestamp := mcdDeepCopy.Annotations[machineutils.LastDeploymentReplicaChangeByScalerTime]
	oldTriggerDeletionAnnot := mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM]
	machineNames := strings.Split(oldTriggerDeletionAnnot, ",")
	newTriggerDeletionAnnotList := make([]string, 0)

	for _, machineName := range machineNames {
		parts := strings.Split(machineName, "~")
		if len(parts) == 1 {
			newTriggerDeletionAnnotList = append(newTriggerDeletionAnnotList, fmt.Sprintf("%s~%s", parts[0], timestamp))
		} else if len(parts) == 2 {
			newTriggerDeletionAnnotList = append(newTriggerDeletionAnnotList, machineName)
		}
	}

	newTriggerDeletionAnnot := strings.Join(newTriggerDeletionAnnotList, ",")
	if oldTriggerDeletionAnnot != newTriggerDeletionAnnot {
		mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM] = newTriggerDeletionAnnot
		newMCD, err := c.controlMachineClient.MachineDeployments(mcd.Namespace).Update(ctx, mcdDeepCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("failed to update MachineDeployment %q with annotation %q=%q", mcdDeepCopy.Name, machineutils.TriggerDeletionByMCM, mcdDeepCopy.Annotations[machineutils.TriggerDeletionByMCM])
			return nil, err
		}
		return newMCD, nil
	}

	return mcdDeepCopy, nil
}

// checkAndAdjustMachineEffectiveCreationTimeout tracks the number of failure replace-cycles
// in which machines failed to join the cluster (tracked via [v1alpha1.AnnotationKeyMachineReplaceCycleCount]),
// and grows the MachineDeployment's effective creation timeout (tracked via [v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout])
// by the configured growth factor each time that count reaches the configured threshold.
//
// The window for counting failures and joins is the period from
// [v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt] to now. When at least
// [constants.DefaultSuccessJoinCountThreshold] machines join successfully within the current window with no
// failures, it shrinks the timeout back to the average observed join duration (floored at the spec creation timeout).
// If no machines have joined or failed for a full max-creation-timeout window, all adjust annotations are cleared
// and the timeout resets to the spec creation timeout.
func (c *controller) checkAndAdjustMachineEffectiveCreationTimeout(ctx context.Context, mcd *v1alpha1.MachineDeployment, machineMap map[types.UID]*v1alpha1.MachineList) (adjusted bool, err error) {
	oldInfo, err := getCreationTimeoutAdjustInfo(mcd)
	if err != nil {
		return
	}
	replaceCycleCountThreshold := GetReplaceCycleCountThresholdOnMachineDeploymentOrDefault(mcd, c.safetyOptions.MachineReplaceCycleCountThreshold)
	creationTimeoutGrowthPercent := GetCreationTimeoutGrowthPercentOnMachineDeploymentOrDefault(mcd, c.safetyOptions.MachineCreationTimeoutGrowthPercent)
	creationTimeoutGrowthFactor := 1.0 + float64(creationTimeoutGrowthPercent)/100.0
	windowStartMark := oldInfo.replaceCycleCountLastAppliedAt.Time
	now := metav1.Now()
	numFailedJoinInWindow, numJoinedInWindow, maxJoinDurationInWindow := getNumFailedJoinedAndMaxJoinDurationSince(flattenMachineMap(machineMap), windowStartMark)
	klog.V(4).Infof("For MachineDeployment %q, numFailedJoinInWindow=%d,numJoinedInWindow=%d,maxJoinDurationInWindow=%s, replaceCount=%d, replaceCycleCountLastAppliedAt: %s",
		mcd.Name, numFailedJoinInWindow, numJoinedInWindow, maxJoinDurationInWindow, oldInfo.replaceCycleCount, oldInfo.replaceCycleCountLastAppliedAt.Time.Format(time.RFC3339))
	specCreationTimeout := GetSpecCreationTimeoutOnMachineDeploymentOrDefault(mcd, constants.DefaultMachineCreationTimeout)
	maxCreationTimeout := computeMaxCreationTimeout(specCreationTimeout, creationTimeoutGrowthFactor, constants.DefaultMaxCreationTimeoutGrowthCount)
	newInfo := oldInfo
	if numFailedJoinInWindow == 0 && oldInfo.isMaxTimeoutElapsed(now, maxCreationTimeout) {
		// No failures and a full max-timeout window has elapsed since the timeout was last applied; clear all adjust annotations so the timeout resets to the spec timeout.
		klog.V(3).Infof("For MachineDeployment %q, clearing all creation-timeout relevant annotations after idle max-creation-timeout window of %q elapsed", mcd.Name, maxCreationTimeout)
		newInfo.reset = true
	} else if numFailedJoinInWindow > 0 && oldInfo.isReplaceCycleCountWindowElapsed(now) {
		// Only increment once per window to avoid multiple increments within the same failure cycle.
		newInfo.replaceCycleCount++
		newInfo.replaceCycleCountLastAppliedAt = now
		if newInfo.replaceCycleCount >= replaceCycleCountThreshold {
			newInfo.effectiveCreationTimeout = increaseEffectiveCreationTimeout(oldInfo.effectiveCreationTimeout, creationTimeoutGrowthFactor, maxCreationTimeout)
			newInfo.effectiveCreationTimeoutLastAppliedAt = now
			newInfo.replaceCycleCount = 0 // reset replace-cycle-count after you grow effective-creation-timeout
			klog.V(3).Infof("For MachineDeployment %q, adjust threshold breached (numFailedJoinInWindow:%d, numJoinedInWindow:%d, replaceCycleCountLastAppliedAt: %s, older replaceCycleCountLastAppliedAt: %s)",
				mcd.Name, numFailedJoinInWindow, numJoinedInWindow, newInfo.replaceCycleCountLastAppliedAt.Format(time.RFC3339), oldInfo.replaceCycleCountLastAppliedAt.Format(time.RFC3339))
		} else {
			klog.V(4).Infof("For MachineDeployment %q, adjust threshold not breached (numFailedJoin:%d, numJoined:%d, replaceCycleCount:%d, replaceCycleCountLastAppliedAt: %s)",
				mcd.Name, numFailedJoinInWindow, numJoinedInWindow, newInfo.replaceCycleCount, newInfo.replaceCycleCountLastAppliedAt.Format(time.RFC3339))
		}
	} else if numJoinedInWindow >= constants.DefaultSuccessJoinCountThreshold && oldInfo.isEffectiveCreationTimeoutWindowElapsed(now) {
		// Machines are joining healthily; thus shrink the timeout back towards the observed average, floored at the spec timeout.
		newInfo.effectiveCreationTimeout.Duration = max(specCreationTimeout.Duration, maxJoinDurationInWindow.Duration)
		newInfo.effectiveCreationTimeoutLastAppliedAt = now
	}
	klog.V(5).Infof("For MachineDeployment %q, old CreationTimeoutAdjustInfo=%s, new CreationTimeoutAdjustInfo=%s", mcd.Name, oldInfo, newInfo)
	modifiedAnnotations, deletedAnnotationKeys := diffCreationTimeoutAnnotations(oldInfo, newInfo)
	if len(modifiedAnnotations) == 0 && len(deletedAnnotationKeys) == 0 {
		return
	}
	newMcd := mcd.DeepCopy()
	for k, v := range modifiedAnnotations {
		metav1.SetMetaDataAnnotation(&newMcd.ObjectMeta, k, v)
	}
	for _, k := range deletedAnnotationKeys {
		delete(newMcd.Annotations, k)
	}
	_, err = c.controlMachineClient.MachineDeployments(mcd.Namespace).Update(ctx, newMcd, metav1.UpdateOptions{})
	if err != nil {
		return
	}
	adjusted = true
	if len(deletedAnnotationKeys) > 0 {
		klog.V(3).Infof("For MachineDeployment %q, cleared all creation-timeout relevant annotations after max-creation-timeout window of %q elapsed", mcd.Name, maxCreationTimeout)
	} else {
		klog.V(3).Infof("For MachineDeployment %q, adjusted creation-timeout relevant annotations: %q", mcd.Name, modifiedAnnotations)
	}
	return
}

// diffCreationTimeoutAnnotations computes the annotation changes needed given the old and new creationTimeoutAdjustInfo.
// It returns modifiedAnnotations (keys to set) and deletedAnnotationKeys (keys to remove).
func diffCreationTimeoutAnnotations(oldInfo, newInfo creationTimeoutAdjustInfo) (modifiedAnnotations map[string]string, deletedAnnotationKeys []string) {
	modifiedAnnotations = make(map[string]string)
	if newInfo.reset {
		deletedAnnotationKeys = []string{
			v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout,
			v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt,
			v1alpha1.AnnotationKeyMachineReplaceCycleCount,
			v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt,
		}
		return
	}
	if oldInfo.replaceCycleCount != newInfo.replaceCycleCount {
		countStr := strconv.FormatInt(int64(newInfo.replaceCycleCount), 10)
		lastAppliedAt := newInfo.replaceCycleCountLastAppliedAt.Time.Format(time.RFC3339)
		modifiedAnnotations[v1alpha1.AnnotationKeyMachineReplaceCycleCount] = countStr
		modifiedAnnotations[v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt] = lastAppliedAt
	}
	if oldInfo.effectiveCreationTimeout.Duration != newInfo.effectiveCreationTimeout.Duration {
		durationStr := newInfo.effectiveCreationTimeout.Duration.String()
		lastAppliedAt := newInfo.effectiveCreationTimeoutLastAppliedAt.Time.Format(time.RFC3339)
		modifiedAnnotations[v1alpha1.AnnotationKeyMachineEffectiveCreationTimeout] = durationStr
		modifiedAnnotations[v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt] = lastAppliedAt
	}
	return
}

func getCreationTimeoutAdjustInfo(mcd *v1alpha1.MachineDeployment) (adjustInfo creationTimeoutAdjustInfo, err error) {
	adjustInfo.effectiveCreationTimeout, err = GetEffectiveCreationTimeoutOnMachineDeployment(mcd)
	if err != nil {
		klog.Warningf("Failed to get effective-creation-timeout for MachineDeployment %q: %v", mcd.Name, err)
		return
	}
	adjustInfo.effectiveCreationTimeoutLastAppliedAt, err = annotations.GetMachineEffectiveCreationTimeoutLastAppliedAt(mcd)
	if err != nil {
		klog.Warningf("Failed to get annotation %q on MachineDeployment %q: %v", v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt, mcd.Name, err)
		return
	}
	adjustInfo.replaceCycleCountLastAppliedAt, err = annotations.GetMachineReplaceCycleCountLastAppliedAt(mcd)
	if err != nil {
		klog.Warningf("Failed to get annotation %q on MachineDeployment %q: %v", v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt, mcd.Name, err)
		return
	}
	if adjustInfo.replaceCycleCountLastAppliedAt.IsZero() {
		adjustInfo.replaceCycleCountLastAppliedAt = mcd.CreationTimestamp
	}
	adjustInfo.replaceCycleCount, err = annotations.GetMachineReplaceCycleCount(mcd)
	if err != nil {
		klog.Warningf("Failed to get annotation %q on MachineDeployment %q: %v", v1alpha1.AnnotationKeyMachineReplaceCycleCount, mcd.Name, err)
		return
	}
	return
}

func flattenMachineMap(machineMap map[types.UID]*v1alpha1.MachineList) []v1alpha1.Machine {
	var machines []v1alpha1.Machine
	for mList := range maps.Values(machineMap) {
		machines = append(machines, mList.Items...)
	}
	return machines
}

// getNumFailedJoinedAndMaxJoinDurationSince returns the number of machines that failed to join and the number
// that successfully joined since windowStartMark, along with the maximum join duration of the successful ones.
func getNumFailedJoinedAndMaxJoinDurationSince(machines []v1alpha1.Machine, windowStartMark time.Time) (numFailedInWindow int, numJoinedInWindow int, maxJoinDurationInWindow metav1.Duration) {
	for _, m := range machines {
		machineJoinedCond := nodeops.FilterNodeConditionOfType(m.Status.Conditions, v1alpha1.ConditionMachineJoined)
		if machineJoinedCond == nil {
			continue
		}
		if machineJoinedCond.Status == v1.ConditionTrue {
			if machineJoinedCond.LastTransitionTime.After(windowStartMark) {
				numJoinedInWindow++
				joinDuration := machineJoinedCond.LastTransitionTime.Sub(m.CreationTimestamp.Time).Round(time.Second)
				if joinDuration > maxJoinDurationInWindow.Duration {
					maxJoinDurationInWindow.Duration = joinDuration
				}
			}
		} else if machineJoinedCond.Status == v1.ConditionFalse &&
			machineJoinedCond.LastTransitionTime.After(windowStartMark) &&
			machineJoinedCond.Reason == codes.FailedJoin.String() {
			numFailedInWindow++
		}
	}
	return
}

// computeMaxCreationTimeout returns the ceiling timeout: `specTimeout * (1 + growthPercent/100)^maxGrowthCount`.
func computeMaxCreationTimeout(specTimeout metav1.Duration, growthFactor float64, maxGrowthCount int) metav1.Duration {
	maxTimeout := specTimeout.Duration
	for range maxGrowthCount {
		maxTimeout = time.Duration(float64(maxTimeout) * growthFactor)
	}
	return metav1.Duration{Duration: maxTimeout.Round(time.Second)}
}

// increaseEffectiveCreationTimeout increases `currTimeout` by `growthFactor` (derived from growthPercent as `1 + growthPercent/100`), capped at `maxCreationTimeout`.
// Returns `currTimeout` unchanged if the cap is already reached or invalid values are specified.
func increaseEffectiveCreationTimeout(currTimeout metav1.Duration, growthFactor float64, maxCreationTimeout metav1.Duration) metav1.Duration {
	if growthFactor <= 1.0 || currTimeout.Duration <= 0 || maxCreationTimeout.Duration <= 0 {
		return currTimeout
	}
	if currTimeout.Duration >= maxCreationTimeout.Duration {
		return currTimeout
	}
	newDuration := time.Duration(float64(currTimeout.Duration) * growthFactor).Round(time.Second)
	if newDuration > maxCreationTimeout.Duration {
		newDuration = maxCreationTimeout.Duration
	}
	return metav1.Duration{Duration: newDuration}
}

type creationTimeoutAdjustInfo struct {
	effectiveCreationTimeout              metav1.Duration
	effectiveCreationTimeoutLastAppliedAt metav1.Time
	replaceCycleCount                     int32
	replaceCycleCountLastAppliedAt        metav1.Time
	// reset indicates that all adjust annotations should be removed, resetting the effective-creation-timeout to the spec timeout.
	reset bool
}

// isReplaceCycleCountWindowElapsed returns true if a full effectiveCreationTimeout window has elapsed since
// [v1alpha1.AnnotationKeyMachineReplaceCycleCountLastAppliedAt].
func (i creationTimeoutAdjustInfo) isReplaceCycleCountWindowElapsed(now metav1.Time) bool {
	return now.Sub(i.replaceCycleCountLastAppliedAt.Time) > i.effectiveCreationTimeout.Duration
}

// isEffectiveCreationTimeoutWindowElapsed returns true if the effectiveCreationTimeout was previously applied
// (i.e. [v1alpha1.AnnotationKeyMachineEffectiveCreationTimeoutLastAppliedAt] is set) and a full effectiveCreationTimeout
// window has elapsed since then.
func (i creationTimeoutAdjustInfo) isEffectiveCreationTimeoutWindowElapsed(now metav1.Time) bool {
	return !i.effectiveCreationTimeoutLastAppliedAt.IsZero() && now.Sub(i.effectiveCreationTimeoutLastAppliedAt.Time) > i.effectiveCreationTimeout.Duration
}

// isMaxTimeoutElapsed returns true if an effective-creation-timeout is set and the full max-creation-timeout
// window has elapsed since it was last applied.
func (i creationTimeoutAdjustInfo) isMaxTimeoutElapsed(now metav1.Time, maxCreationTimeout metav1.Duration) bool {
	if i.effectiveCreationTimeoutLastAppliedAt.IsZero() {
		return false
	}
	return now.Sub(i.effectiveCreationTimeoutLastAppliedAt.Time) > maxCreationTimeout.Duration
}

func (i creationTimeoutAdjustInfo) String() string {
	return fmt.Sprintf("(effectiveCreationTimeout=%q, effectiveCreationTimeoutLastAppliedAt=%q, replaceCycleCount=%d, replaceCycleCountLastAppliedAt=%q)",
		i.effectiveCreationTimeout,
		i.effectiveCreationTimeoutLastAppliedAt.Time.Format(time.RFC3339),
		i.replaceCycleCount,
		i.replaceCycleCountLastAppliedAt.Time.Format(time.RFC3339))
}
