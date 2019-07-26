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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/rollback.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"time"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// rollback the deployment to the specified revision. In any case cleanup the rollback spec.
func (dc *controller) rollback(d *v1alpha1.MachineDeployment, isList []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) error {
	newIS, allOldISs, err := dc.getAllMachineSetsAndSyncRevision(d, isList, machineMap, true)
	if err != nil {
		return err
	}

	allISs := append(allOldISs, newIS)
	toRevision := &d.Spec.RollbackTo.Revision
	// If rollback revision is 0, rollback to the last revision
	if *toRevision == 0 {
		if *toRevision = LastRevision(allISs); *toRevision == 0 {
			// If we still can't find the last revision, gives up rollback
			dc.emitRollbackWarningEvent(d, RollbackRevisionNotFound, "Unable to find last revision.")
			// Gives up rollback
			return dc.updateMachineDeploymentAndClearRollbackTo(d)
		}
	}
	for _, is := range allISs {
		v, err := Revision(is)
		if err != nil {
			glog.V(4).Infof("Unable to extract revision from deployment's machine set %q: %v", is.Name, err)
			continue
		}
		if v == *toRevision {
			glog.V(4).Infof("Found machine set %q with desired revision %d", is.Name, v)

			// Remove PreferNoSchedule taints from nodes which were backing the machineSet
			err = dc.removeTaintNodesBackingMachineSet(
				is,
				&v1.Taint{
					Key:    PreferNoScheduleKey,
					Value:  "True",
					Effect: "PreferNoSchedule",
				},
			)
			if err != nil {
				glog.Warningf("Failed to remove taints %s off nodes. Error: %s", PreferNoScheduleKey, err)
			}

			// rollback by copying podTemplate.Spec from the machine set
			// revision number will be incremented during the next getAllMachineSetsAndSyncRevision call
			// no-op if the spec matches current deployment's podTemplate.Spec
			performedRollback, err := dc.rollbackToTemplate(d, is)
			if performedRollback && err == nil {
				dc.emitRollbackNormalEvent(d, fmt.Sprintf("Rolled back deployment %q to revision %d", d.Name, *toRevision))
			}
			return err
		}
	}
	dc.emitRollbackWarningEvent(d, RollbackRevisionNotFound, "Unable to find the revision to rollback to.")
	// Gives up rollback
	return dc.updateMachineDeploymentAndClearRollbackTo(d)
}

// rollbackToTemplate compares the templates of the provided deployment and machine set and
// updates the deployment with the machine set template in case they are different. It also
// cleans up the rollback spec so subsequent requeues of the deployment won't end up in here.
func (dc *controller) rollbackToTemplate(d *v1alpha1.MachineDeployment, is *v1alpha1.MachineSet) (bool, error) {
	performedRollback := false
	if !EqualIgnoreHash(&d.Spec.Template, &is.Spec.Template) {
		glog.V(4).Infof("Rolling back deployment %q to template spec %+v", d.Name, is.Spec.Template.Spec)
		SetFromMachineSetTemplate(d, is.Spec.Template)
		// set RS (the old RS we'll rolling back to) annotations back to the deployment;
		// otherwise, the deployment's current annotations (should be the same as current new RS) will be copied to the RS after the rollback.
		//
		// For example,
		// A Deployment has old RS1 with annotation {change-cause:create}, and new RS2 {change-cause:edit}.
		// Note that both annotations are copied from Deployment, and the Deployment should be annotated {change-cause:edit} as well.
		// Now, rollback Deployment to RS1, we should update Deployment's pod-template and also copy annotation from RS1.
		// Deployment is now annotated {change-cause:create}, and we have new RS1 {change-cause:create}, old RS2 {change-cause:edit}.
		//
		// If we don't copy the annotations back from RS to deployment on rollback, the Deployment will stay as {change-cause:edit},
		// and new RS1 becomes {change-cause:edit} (copied from deployment after rollback), old RS2 {change-cause:edit}, which is not correct.
		SetMachineDeploymentAnnotationsTo(d, is)
		performedRollback = true
	} else {
		glog.V(4).Infof("Rolling back to a revision that contains the same template as current deployment %q, skipping rollback...", d.Name)
		eventMsg := fmt.Sprintf("The rollback revision contains the same template as current deployment %q", d.Name)
		dc.emitRollbackWarningEvent(d, RollbackTemplateUnchanged, eventMsg)
	}

	return performedRollback, dc.updateMachineDeploymentAndClearRollbackTo(d)
}

func (dc *controller) emitRollbackWarningEvent(d *v1alpha1.MachineDeployment, reason, message string) {
	dc.recorder.Eventf(d, v1.EventTypeWarning, reason, message)
}

func (dc *controller) emitRollbackNormalEvent(d *v1alpha1.MachineDeployment, message string) {
	dc.recorder.Eventf(d, v1.EventTypeNormal, RollbackDone, message)
}

// updateDeploymentAndClearRollbackTo sets .spec.rollbackTo to nil and update the input deployment
// It is assumed that the caller will have updated the deployment template appropriately (in case
// we want to rollback).
func (dc *controller) updateMachineDeploymentAndClearRollbackTo(d *v1alpha1.MachineDeployment) error {
	glog.V(4).Infof("Cleans up rollbackTo of machine deployment %q", d.Name)
	d.Spec.RollbackTo = nil
	_, err := dc.controlMachineClient.MachineDeployments(d.Namespace).Update(d)
	return err
}

// removeTaintNodesBackingMachineSet removes taints from all nodes backing the machineSets
func (dc *controller) removeTaintNodesBackingMachineSet(machineSet *v1alpha1.MachineSet, taint *v1.Taint) error {

	if _, exists := machineSet.Annotations[taint.Key]; !exists {
		// No taint exists
		glog.Warningf("No taint exists on machineSet: %s. Hence not removing.", machineSet.Name)
		return nil
	}

	glog.V(2).Infof("Trying to untaint MachineSet object %q with %s to enable scheduling of pods", machineSet.Name, taint.Key)
	selector, err := metav1.LabelSelectorAsSelector(machineSet.Spec.Selector)
	if err != nil {
		return err
	}

	// list all machines to include the machines that don't match the ms`s selector
	// anymore but has the stale controller ref.
	// TODO: Do the List and Filter in a single pass, or use an index.
	filteredMachines, err := dc.machineLister.List(labels.Everything())
	if err != nil {
		return err
	}
	// NOTE: filteredMachines are pointing to objects from cache - if you need to
	// modify them, you need to copy it first.
	filteredMachines, err = dc.claimMachines(machineSet, selector, filteredMachines)
	if err != nil {
		return err
	}

	// Iterate through all machines and remove the PreferNoSchedule taint
	// to avoid scheduling on older machines
	for _, machine := range filteredMachines {
		if machine.Status.Node != "" {
			node, err := dc.targetCoreClient.CoreV1().Nodes().Get(machine.Status.Node, metav1.GetOptions{})
			if err != nil {
				glog.Warningf("Node taint removal failed for node: %s, Error: %s", machine.Status.Node, err)
				continue
			}

			err = RemoveTaintOffNode(
				dc.targetCoreClient,
				machine.Status.Node,
				node,
				taint,
			)
			if err != nil {
				glog.Warningf("Node taint removal failed for node: %s, Error: %s", machine.Status.Node, err)
			}
			node, err = dc.targetCoreClient.CoreV1().Nodes().Get(machine.Status.Node, metav1.GetOptions{})
		}
	}

	retryDeadline := time.Now().Add(maxRetryDeadline)
	for {
		machineSet, err = dc.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
		if err != nil {
			if time.Now().Before(retryDeadline) {
				glog.Warningf("Unable to fetch MachineSet object %s, Error: %+v", machineSet.Name, err)
				time.Sleep(conflictRetryInterval)
				continue
			} else {
				// Timeout occurred
				glog.Errorf("Timeout occurred: Unable to fetch MachineSet object %s, Error: %+v", machineSet.Name, err)
				return err
			}
		}

		msCopy := machineSet.DeepCopy()
		delete(msCopy.Annotations, taint.Key)

		machineSet, err = dc.controlMachineClient.MachineSets(msCopy.Namespace).Update(msCopy)

		if err != nil {
			if time.Now().Before(retryDeadline) {
				glog.Warningf("Unable to update MachineSet object %s, Error: %+v", machineSet.Name, err)
				time.Sleep(conflictRetryInterval)
				continue
			} else {
				// Timeout occurred
				glog.Errorf("Timeout occurred: Unable to update MachineSet object %s, Error: %+v", machineSet.Name, err)
				return err
			}
		}

		// Break out of loop when update succeeds
		break
	}
	glog.V(2).Infof("Removed taint %s from MachineSet object %q", taint.Key, machineSet.Name)

	return nil
}
