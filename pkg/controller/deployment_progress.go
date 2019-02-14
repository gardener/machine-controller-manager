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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/progress.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

// syncRolloutStatus updates the status of a deployment during a rollout. There are
// cases this helper will run that cannot be prevented from the scaling detection,
// for example a resync of the deployment after it was scaled up. In those cases,
// we shouldn't try to estimate any progress.
func (dc *controller) syncRolloutStatus(allISs []*v1alpha1.MachineSet, newIS *v1alpha1.MachineSet, d *v1alpha1.MachineDeployment) error {
	newStatus := calculateDeploymentStatus(allISs, newIS, d)

	// If there is no progressDeadlineSeconds set, remove any Progressing condition.
	if d.Spec.ProgressDeadlineSeconds == nil {
		RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentProgressing)
	}

	// If there is only one machine set that is active then that means we are not running
	// a new rollout and this is a resync where we don't need to estimate any progress.
	// In such a case, we should simply not estimate any progress for this deployment.
	currentCond := GetMachineDeploymentCondition(d.Status, v1alpha1.MachineDeploymentProgressing)
	isCompleteDeployment := newStatus.Replicas == newStatus.UpdatedReplicas && currentCond != nil && currentCond.Reason == NewISAvailableReason
	// Check for progress only if there is a progress deadline set and the latest rollout
	// hasn't completed yet.
	if d.Spec.ProgressDeadlineSeconds != nil && !isCompleteDeployment {
		switch {
		case MachineDeploymentComplete(d, &newStatus):
			// Update the deployment conditions with a message for the new machine set that
			// was successfully deployed. If the condition already exists, we ignore this update.
			msg := fmt.Sprintf("Machine Deployment %q has successfully progressed.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("MachineSet %q has successfully progressed.", newIS.Name)
			}
			condition := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentProgressing, v1alpha1.ConditionTrue, NewISAvailableReason, msg)
			SetMachineDeploymentCondition(&newStatus, *condition)

		case MachineDeploymentProgressing(d, &newStatus):
			// If there is any progress made, continue by not checking if the deployment failed. This
			// behavior emulates the rolling updater progressDeadline check.
			msg := fmt.Sprintf("Machine Deployment %q is progressing.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("MachineSet %q is progressing.", newIS.Name)
			}
			condition := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentProgressing, v1alpha1.ConditionTrue, MachineSetUpdatedReason, msg)
			// Update the current Progressing condition or add a new one if it doesn't exist.
			// If a Progressing condition with status=true already exists, we should update
			// everything but lastTransitionTime. SetDeploymentCondition already does that but
			// it also is not updating conditions when the reason of the new condition is the
			// same as the old. The Progressing condition is a special case because we want to
			// update with the same reason and change just lastUpdateTime iff we notice any
			// progress. That's why we handle it here.
			if currentCond != nil {
				if currentCond.Status == v1alpha1.ConditionTrue {
					condition.LastTransitionTime = currentCond.LastTransitionTime
				}
				RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentProgressing)
			}
			SetMachineDeploymentCondition(&newStatus, *condition)

		case MachineDeploymentTimedOut(d, &newStatus):
			// Update the deployment with a timeout condition. If the condition already exists,
			// we ignore this update.
			msg := fmt.Sprintf("Machine Deployment %q has timed out progressing.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("MachineSet %q has timed out progressing.", newIS.Name)
			}
			condition := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentProgressing, v1alpha1.ConditionFalse, TimedOutReason, msg)
			SetMachineDeploymentCondition(&newStatus, *condition)
		}
	}

	// Move failure conditions of all machine sets in deployment conditions. For now,
	// only one failure condition is returned from getReplicaFailures.
	if replicaFailureCond := dc.getReplicaFailures(allISs, newIS); len(replicaFailureCond) > 0 {
		// There will be only one ReplicaFailure condition on the machine set.
		SetMachineDeploymentCondition(&newStatus, replicaFailureCond[0])
	} else {
		RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentReplicaFailure)
	}

	// Do not update if there is nothing new to add.
	if !statusUpdateRequired(d.Status, newStatus) {
		// Requeue the deployment if required.
		dc.requeueStuckMachineDeployment(d, newStatus)
		return nil
	}

	newDeployment := d
	newDeployment.Status = newStatus
	_, err := dc.controlMachineClient.MachineDeployments(newDeployment.Namespace).UpdateStatus(newDeployment)
	return err
}

// getReplicaFailures will convert replica failure conditions from machine sets
// to deployment conditions.
func (dc *controller) getReplicaFailures(allISs []*v1alpha1.MachineSet, newIS *v1alpha1.MachineSet) []v1alpha1.MachineDeploymentCondition {
	var conditions []v1alpha1.MachineDeploymentCondition
	if newIS != nil {
		for _, c := range newIS.Status.Conditions {
			if c.Type != v1alpha1.MachineSetReplicaFailure {
				continue
			}
			conditions = append(conditions, MachineSetToMachineDeploymentCondition(c))
		}
	}

	// Return failures for the new machine set over failures from old machine sets.
	if len(conditions) > 0 {
		return conditions
	}

	for i := range allISs {
		is := allISs[i]
		if is == nil {
			continue
		}

		for _, c := range is.Status.Conditions {
			if c.Type != v1alpha1.MachineSetReplicaFailure {
				continue
			}
			conditions = append(conditions, MachineSetToMachineDeploymentCondition(c))
		}
	}
	return conditions
}

// // used for unit testing
// var nowFn = func() time.Time { return time.Now() }

// requeueStuckDeployment checks whether the provided deployment needs to be synced for a progress
// check. It returns the time after the deployment will be requeued for the progress check, 0 if it
// will be requeued now, or -1 if it does not need to be requeued.
func (dc *controller) requeueStuckMachineDeployment(d *v1alpha1.MachineDeployment, newStatus v1alpha1.MachineDeploymentStatus) time.Duration {
	currentCond := GetMachineDeploymentCondition(d.Status, v1alpha1.MachineDeploymentProgressing)
	// Can't estimate progress if there is no deadline in the spec or progressing condition in the current status.
	if d.Spec.ProgressDeadlineSeconds == nil || currentCond == nil {
		return time.Duration(-1)
	}
	// No need to estimate progress if the rollout is complete or already timed out.
	if MachineDeploymentComplete(d, &newStatus) || currentCond.Reason == TimedOutReason {
		return time.Duration(-1)
	}
	// If there is no sign of progress at this point then there is a high chance that the
	// deployment is stuck. We should resync this deployment at some point in the future[1]
	// and check whether it has timed out. We definitely need this, otherwise we depend on the
	// controller resync interval. See https://github.com/kubernetes/kubernetes/issues/34458.
	//
	// [1] ProgressingCondition.LastUpdatedTime + progressDeadlineSeconds - time.Now()
	//
	// For example, if a Deployment updated its Progressing condition 3 minutes ago and has a
	// deadline of 10 minutes, it would need to be resynced for a progress check after 7 minutes.
	//
	// lastUpdated: 			00:00:00
	// now: 					00:03:00
	// progressDeadlineSeconds: 600 (10 minutes)
	//
	// lastUpdated + progressDeadlineSeconds - now => 00:00:00 + 00:10:00 - 00:03:00 => 07:00
	after := currentCond.LastUpdateTime.Time.Add(time.Duration(*d.Spec.ProgressDeadlineSeconds) * time.Second).Sub(nowFn())
	// If the remaining time is less than a second, then requeue the deployment immediately.
	// Make it ratelimited so we stay on the safe side, eventually the Deployment should
	// transition either to a Complete or to a TimedOut condition.
	if after < time.Second {
		glog.V(4).Infof("Queueing up machine deployment %q for a progress check now", d.Name)
		dc.enqueueRateLimited(d)
		return time.Duration(0)
	}
	glog.V(4).Infof("Queueing up machine deployment %q for a progress check after %ds", d.Name, int(after.Seconds()))
	// Add a second to avoid milliseconds skew in AddAfter.
	// See https://github.com/kubernetes/kubernetes/issues/39785#issuecomment-279959133 for more info.
	dc.enqueueMachineDeploymentAfter(d, after+time.Second)
	return after
}
