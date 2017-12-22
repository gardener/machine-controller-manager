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

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"fmt"
	"reflect"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	nodeclientset "code.sapcloud.io/kubernetes/node-controller-manager/pkg/client/clientset/typed/node/v1alpha1"
)

// updateInstanceSetStatus attempts to update the Status.Replicas of the given InstanceSet, with a single GET/PUT retry.
func updateInstanceSetStatus(nodeClient nodeclientset.NodeV1alpha1Interface, is *v1alpha1.InstanceSet, newStatus v1alpha1.InstanceSetStatus) (*v1alpha1.InstanceSet, error) {
	// This is the steady state. It happens when the InstanceSet doesn't have any expectations, since
	// we do a periodic relist every 30s. If the generations differ but the replicas are
	// the same, a caller might've resized to the same replica count.
	c := nodeClient.InstanceSets()

	if is.Status.Replicas == newStatus.Replicas &&
		is.Status.FullyLabeledReplicas == newStatus.FullyLabeledReplicas &&
		is.Status.ReadyReplicas == newStatus.ReadyReplicas &&
		is.Status.AvailableReplicas == newStatus.AvailableReplicas &&
		is.Generation == is.Status.ObservedGeneration &&
		reflect.DeepEqual(is.Status.Conditions, newStatus.Conditions) {
		return is, nil
	}

	// Save the generation number we acted on, otherwise we might wrongfully indicate
	// that we've seen a spec update when we retry.
	// TODO: This can clobber an update if we allow multiple agents to write to the
	// same status.
	newStatus.ObservedGeneration = is.Generation

	var getErr, updateErr error
	var updatedIS *v1alpha1.InstanceSet
	for i, is := 0, is; ; i++ {
		glog.V(4).Infof(fmt.Sprintf("Updating status for InstanceSet: %s/%s, ", is.Namespace, is.Name) +
			fmt.Sprintf("replicas %d->%d (need %d), ", is.Status.Replicas, newStatus.Replicas, is.Spec.Replicas) +
			fmt.Sprintf("fullyLabeledReplicas %d->%d, ", is.Status.FullyLabeledReplicas, newStatus.FullyLabeledReplicas) +
			fmt.Sprintf("readyReplicas %d->%d, ", is.Status.ReadyReplicas, newStatus.ReadyReplicas) +
			fmt.Sprintf("availableReplicas %d->%d, ", is.Status.AvailableReplicas, newStatus.AvailableReplicas) +
			fmt.Sprintf("sequence No: %v->%v", is.Status.ObservedGeneration, newStatus.ObservedGeneration))

		is.Status = newStatus
		updatedIS, updateErr = c.Update(is)

		if updateErr == nil {
			return updatedIS, nil
		}
		// Stop retrying if we exceed statusUpdateRetries - the InstanceSet will be requeued with a rate limit.
		if i >= statusUpdateRetries {
			break
		}
		// Update the InstanceSet with the latest resource veision for the next poll
		if is, getErr = c.Get(is.Name, metav1.GetOptions{}); getErr != nil {
			// If the GET fails we can't trust status.Replicas anymore. This error
			// is bound to be more interesting than the update failure.
			return nil, getErr
		}
	}

	return nil, updateErr
}

func calculateInstanceSetStatus(is *v1alpha1.InstanceSet, filteredInstances []*v1alpha1.Instance, manageReplicasErr error) v1alpha1.InstanceSetStatus {
	newStatus := is.Status
	// Count the number of instances that have labels matching the labels of the instance
	// template of the instance set, the matching instances may have more
	// labels than are in the template. Because the label of instanceTemplateSpec is
	// a supeiset of the selector of the instance set, so the possible
	// matching instances must be part of the filteredinstances.
	fullyLabeledReplicasCount := 0
	readyReplicasCount := 0
	availableReplicasCount := 0

	templateLabel := labels.Set(is.Spec.Template.Labels).AsSelectorPreValidated()
	for _, instance := range filteredInstances {
		if templateLabel.Matches(labels.Set(instance.Labels)) {
			fullyLabeledReplicasCount++
		}
		if isInstanceAvailable(instance) {
			availableReplicasCount++
			if isInstanceReady(instance) {
				readyReplicasCount++
			}
		}
	}

	failureCond := GetCondition(&is.Status, v1alpha1.InstanceSetReplicaFailure)
	if manageReplicasErr != nil && failureCond == nil {
		var reason string
		if diff := len(filteredInstances) - int(is.Spec.Replicas); diff < 0 {
			reason = "FailedCreate"
		} else if diff > 0 {
			reason = "FailedDelete"
		}
		cond := NewInstanceSetCondition(v1alpha1.InstanceSetReplicaFailure, v1alpha1.ConditionTrue, reason, manageReplicasErr.Error())
		SetCondition(&newStatus, cond)
	} else if manageReplicasErr == nil && failureCond != nil {
		RemoveCondition(&newStatus, v1alpha1.InstanceSetReplicaFailure)
	}

	newStatus.Replicas = int32(len(filteredInstances))
	newStatus.FullyLabeledReplicas = int32(fullyLabeledReplicasCount)
	newStatus.ReadyReplicas = int32(readyReplicasCount)
	newStatus.AvailableReplicas = int32(availableReplicasCount)
	return newStatus
}

// NewInstanceSetCondition creates a new InstanceSet condition.
func NewInstanceSetCondition(condType v1alpha1.InstanceSetConditionType, status v1alpha1.ConditionStatus, reason, msg string) v1alpha1.InstanceSetCondition {
	return v1alpha1.InstanceSetCondition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            msg,
	}
}

// GetCondition returns a InstanceSet condition with the provided type if it exists.
func GetCondition(status *v1alpha1.InstanceSetStatus, condition v1alpha1.InstanceSetConditionType) *v1alpha1.InstanceSetCondition {
	for _, c := range status.Conditions {
		if c.Type == condition {
			return &c
		}
	}
	return nil
}

// SetCondition adds/replaces the given condition in the InstanceSet status. If the condition that we
// are about to add already exists and has the same status and reason then we are not going to update.
func SetCondition(status *v1alpha1.InstanceSetStatus, condition v1alpha1.InstanceSetCondition) {
	currentCond := GetCondition(status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	newConditions := filterOutInstanceSetCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveCondition removes the condition with the provided type from the InstanceSet status.
func RemoveCondition(status *v1alpha1.InstanceSetStatus, condType v1alpha1.InstanceSetConditionType) {
	status.Conditions = filterOutInstanceSetCondition(status.Conditions, condType)
}

// filterOutInstanceSetCondition returns a new slice of InstanceSet conditions without conditions with the provided type.
func filterOutInstanceSetCondition(conditions []v1alpha1.InstanceSetCondition, condType v1alpha1.InstanceSetConditionType) []v1alpha1.InstanceSetCondition {
	var newConditions []v1alpha1.InstanceSetCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

func isInstanceAvailable (instance *v1alpha1.Instance) bool {

	if instance.Status.CurrentStatus.Phase == v1alpha1.InstanceAvailable ||
			instance.Status.CurrentStatus.Phase == v1alpha1.InstanceRunning {
		return true
	}

	return false
}

func isInstanceReady (instance *v1alpha1.Instance) bool {

	// TODO add more conditions
	if instance.Status.CurrentStatus.Phase == v1alpha1.InstanceRunning {
		return true
	}
	
	return false
}
