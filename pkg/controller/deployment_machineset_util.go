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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/controller/deployment/util/replicaset_util.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	machineapi "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// updateMachineSetStatus attempts to update the Status.Replicas of the given MachineSet, with a single GET/PUT retry.
func updateMachineSetStatus(machineClient machineapi.MachineV1alpha1Interface, is *v1alpha1.MachineSet, newStatus v1alpha1.MachineSetStatus) (*v1alpha1.MachineSet, error) {
	// This is the steady state. It happens when the MachineSet doesn't have any expectations, since
	// we do a periodic relist every 30s. If the generations differ but the replicas are
	// the same, a caller might've resized to the same replica count.
	c := machineClient.MachineSets(is.Namespace)

	if is.Status.Replicas == newStatus.Replicas &&
		is.Status.FullyLabeledReplicas == newStatus.FullyLabeledReplicas &&
		is.Status.ReadyReplicas == newStatus.ReadyReplicas &&
		is.Status.AvailableReplicas == newStatus.AvailableReplicas &&
		is.Generation == is.Status.ObservedGeneration &&
		reflect.DeepEqual(is.Status.Conditions, newStatus.Conditions) &&
		reflect.DeepEqual(is.Status.FailedMachines, newStatus.FailedMachines) {
		return is, nil
	}

	// Save the generation number we acted on, otherwise we might wrongfully indicate
	// that we've seen a spec update when we retry.
	// TODO: This can clobber an update if we allow multiple agents to write to the
	// same status.
	newStatus.ObservedGeneration = is.Generation

	var getErr, updateErr error
	var updatedIS *v1alpha1.MachineSet
	for i, is := 0, is; ; i++ {
		glog.V(4).Infof(fmt.Sprintf("Updating status for MachineSet: %s/%s, ", is.Namespace, is.Name) +
			fmt.Sprintf("replicas %d->%d (need %d), ", is.Status.Replicas, newStatus.Replicas, is.Spec.Replicas) +
			fmt.Sprintf("fullyLabeledReplicas %d->%d, ", is.Status.FullyLabeledReplicas, newStatus.FullyLabeledReplicas) +
			fmt.Sprintf("readyReplicas %d->%d, ", is.Status.ReadyReplicas, newStatus.ReadyReplicas) +
			fmt.Sprintf("availableReplicas %d->%d, ", is.Status.AvailableReplicas, newStatus.AvailableReplicas) +
			fmt.Sprintf("sequence No: %v->%v", is.Status.ObservedGeneration, newStatus.ObservedGeneration))

		is.Status = newStatus
		updatedIS, updateErr = c.UpdateStatus(is)

		if updateErr == nil {
			return updatedIS, nil
		}
		// Stop retrying if we exceed statusUpdateRetries - the MachineSet will be requeued with a rate limit.
		if i >= statusUpdateRetries {
			break
		}
		// Update the MachineSet with the latest resource veision for the next poll
		if is, getErr = c.Get(is.Name, metav1.GetOptions{}); getErr != nil {
			// If the GET fails we can't trust status.Replicas anymore. This error
			// is bound to be more interesting than the update failure.
			return nil, getErr
		}
	}

	return nil, updateErr
}

func calculateMachineSetStatus(is *v1alpha1.MachineSet, filteredMachines []*v1alpha1.Machine, manageReplicasErr error) v1alpha1.MachineSetStatus {
	newStatus := is.Status
	// Count the number of machines that have labels matching the labels of the machine
	// template of the machine set, the matching machines may have more
	// labels than are in the template. Because the label of machineTemplateSpec is
	// a supeiset of the selector of the machine set, so the possible
	// matching machines must be part of the filteredmachines.
	fullyLabeledReplicasCount := 0
	readyReplicasCount := 0
	availableReplicasCount := 0

	failedMachines := []v1alpha1.MachineSummary{}
	var machineSummary v1alpha1.MachineSummary

	templateLabel := labels.Set(is.Spec.Template.Labels).AsSelectorPreValidated()
	for _, machine := range filteredMachines {
		if templateLabel.Matches(labels.Set(machine.Labels)) {
			fullyLabeledReplicasCount++
		}
		if isMachineAvailable(machine) {
			availableReplicasCount++
			if isMachineReady(machine) {
				readyReplicasCount++
			}
		}
		if machine.Status.LastOperation.State == v1alpha1.MachineStateFailed {
			machineSummary.Name = machine.Name
			machineSummary.ProviderID = machine.Spec.ProviderID
			machineSummary.LastOperation = machine.Status.LastOperation
			//ownerRef populated here, so that deployment controller doesn't have to add it separately
			if controller := metav1.GetControllerOf(machine); controller != nil {
				machineSummary.OwnerRef = controller.Name
			}
			failedMachines = append(failedMachines, machineSummary)
		}
	}

	// Update the FailedMachines field only if we see new failures
	// Clear FailedMachines if ready replicas equals total replicas,
	// which means the machineset doesn't have any machine objects which are in any failed state
	if len(failedMachines) > 0 {
		newStatus.FailedMachines = &failedMachines
	} else if int32(readyReplicasCount) == is.Status.Replicas {
		newStatus.FailedMachines = nil
	}

	failureCond := GetCondition(&is.Status, v1alpha1.MachineSetReplicaFailure)
	if manageReplicasErr != nil && failureCond == nil {
		var reason string
		if diff := len(filteredMachines) - int(is.Spec.Replicas); diff < 0 {
			reason = "FailedCreate"
		} else if diff > 0 {
			reason = "FailedDelete"
		}
		cond := NewMachineSetCondition(v1alpha1.MachineSetReplicaFailure, v1alpha1.ConditionTrue, reason, manageReplicasErr.Error())
		SetCondition(&newStatus, cond)
	} else if manageReplicasErr == nil && failureCond != nil {
		RemoveCondition(&newStatus, v1alpha1.MachineSetReplicaFailure)
	}

	newStatus.Replicas = int32(len(filteredMachines))
	newStatus.FullyLabeledReplicas = int32(fullyLabeledReplicasCount)
	newStatus.ReadyReplicas = int32(readyReplicasCount)
	newStatus.AvailableReplicas = int32(availableReplicasCount)
	return newStatus
}

// NewMachineSetCondition creates a new MachineSet condition.
func NewMachineSetCondition(condType v1alpha1.MachineSetConditionType, status v1alpha1.ConditionStatus, reason, msg string) v1alpha1.MachineSetCondition {
	return v1alpha1.MachineSetCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// GetCondition returns a MachineSet condition with the provided type if it exists.
func GetCondition(status *v1alpha1.MachineSetStatus, condition v1alpha1.MachineSetConditionType) *v1alpha1.MachineSetCondition {
	for _, c := range status.Conditions {
		if c.Type == condition {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the MachineSet status. If the condition that we
// are about to add already exists and has the same status and reason then we are not going to update.
func SetCondition(status *v1alpha1.MachineSetStatus, condition v1alpha1.MachineSetCondition) {
	currentCond := GetCondition(status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	newConditions := filterOutMachineSetCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveCondition removes the condition with the provided type from the MachineSet status.
func RemoveCondition(status *v1alpha1.MachineSetStatus, condType v1alpha1.MachineSetConditionType) {
	status.Conditions = filterOutMachineSetCondition(status.Conditions, condType)
}

// filterOutMachineSetCondition returns a new slice of MachineSet conditions without conditions with the provided type.
func filterOutMachineSetCondition(conditions []v1alpha1.MachineSetCondition, condType v1alpha1.MachineSetConditionType) []v1alpha1.MachineSetCondition {
	var newConditions []v1alpha1.MachineSetCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

func isMachineAvailable(machine *v1alpha1.Machine) bool {

	if machine.Status.CurrentStatus.Phase == v1alpha1.MachineAvailable ||
		machine.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
		return true
	}

	return false
}

func isMachineReady(machine *v1alpha1.Machine) bool {

	// TODO add more conditions
	if machine.Status.CurrentStatus.Phase == v1alpha1.MachineRunning {
		return true
	}

	return false
}
