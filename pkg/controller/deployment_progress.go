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

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"

	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"

)

// syncRolloutStatus updates the status of a deployment during a rollout. There are
// cases this helper will run that cannot be prevented from the scaling detection,
// for example a resync of the deployment after it was scaled up. In those cases,
// we shouldn't try to estimate any progress.
func (dc *controller) syncRolloutStatus(allISs []*v1alpha1.InstanceSet, newIS *v1alpha1.InstanceSet, d *v1alpha1.InstanceDeployment) error {
	newStatus := calculateDeploymentStatus(allISs, newIS, d)

	// If there is no progressDeadlineSeconds set, remove any Progressing condition.
	if d.Spec.ProgressDeadlineSeconds == nil {
		RemoveInstanceDeploymentCondition(&newStatus, v1alpha1.InstanceDeploymentProgressing)
	}

	// If there is only one instance set that is active then that means we are not running
	// a new rollout and this is a resync where we don't need to estimate any progress.
	// In such a case, we should simply not estimate any progress for this deployment.
	currentCond := GetInstanceDeploymentCondition(d.Status, v1alpha1.InstanceDeploymentProgressing)
	isCompleteDeployment := newStatus.Replicas == newStatus.UpdatedReplicas && currentCond != nil && currentCond.Reason == NewISAvailableReason
	// Check for progress only if there is a progress deadline set and the latest rollout
	// hasn't completed yet.
	if d.Spec.ProgressDeadlineSeconds != nil && !isCompleteDeployment {
		switch {
		case InstanceDeploymentComplete(d, &newStatus):
			// Update the deployment conditions with a message for the new instance set that
			// was successfully deployed. If the condition already exists, we ignore this update.
			msg := fmt.Sprintf("Instance Deployment %q has successfully progressed.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("InstanceSet %q has successfully progressed.", newIS.Name)
			}
			condition := NewInstanceDeploymentCondition(v1alpha1.InstanceDeploymentProgressing, v1alpha1.ConditionTrue, NewISAvailableReason, msg)
			SetInstanceDeploymentCondition(&newStatus, *condition)

		case InstanceDeploymentProgressing(d, &newStatus):
			// If there is any progress made, continue by not checking if the deployment failed. This
			// behavior emulates the rolling updater progressDeadline check.
			msg := fmt.Sprintf("Instance Deployment %q is progressing.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("InstanceSet %q is progressing.", newIS.Name)
			}
			condition := NewInstanceDeploymentCondition(v1alpha1.InstanceDeploymentProgressing, v1alpha1.ConditionTrue, InstanceSetUpdatedReason, msg)
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
				RemoveInstanceDeploymentCondition(&newStatus, v1alpha1.InstanceDeploymentProgressing)
			}
			SetInstanceDeploymentCondition(&newStatus, *condition)

		case InstanceDeploymentTimedOut(d, &newStatus):
			// Update the deployment with a timeout condition. If the condition already exists,
			// we ignore this update.
			msg := fmt.Sprintf("Instance Deployment %q has timed out progressing.", d.Name)
			if newIS != nil {
				msg = fmt.Sprintf("InstanceSet %q has timed out progressing.", newIS.Name)
			}
			condition := NewInstanceDeploymentCondition(v1alpha1.InstanceDeploymentProgressing, v1alpha1.ConditionFalse, TimedOutReason, msg)
			SetInstanceDeploymentCondition(&newStatus, *condition)
		}
	}

	// Move failure conditions of all instance sets in deployment conditions. For now,
	// only one failure condition is returned from getReplicaFailures.
	if replicaFailureCond := dc.getReplicaFailures(allISs, newIS); len(replicaFailureCond) > 0 {
		// There will be only one ReplicaFailure condition on the instance set.
		SetInstanceDeploymentCondition(&newStatus, replicaFailureCond[0])
	} else {
		RemoveInstanceDeploymentCondition(&newStatus, v1alpha1.InstanceDeploymentReplicaFailure)
	}

	// Do not update if there is nothing new to add.
	if reflect.DeepEqual(d.Status, newStatus) {
		// Requeue the deployment if required.
		dc.requeueStuckInstanceDeployment(d, newStatus)
		return nil
	}

	newDeployment := d
	newDeployment.Status = newStatus
	_, err := dc.nodeClient.InstanceDeployments().Update(newDeployment)
	return err
}

// getReplicaFailures will convert replica failure conditions from instance sets
// to deployment conditions.
func (dc *controller) getReplicaFailures(allISs []*v1alpha1.InstanceSet, newIS *v1alpha1.InstanceSet) []v1alpha1.InstanceDeploymentCondition {
	var conditions []v1alpha1.InstanceDeploymentCondition
	if newIS != nil {
		for _, c := range newIS.Status.Conditions {
			if c.Type != v1alpha1.InstanceSetReplicaFailure {
				continue
			}
			conditions = append(conditions, InstanceSetToInstanceDeploymentCondition(c))
		}
	}

	// Return failures for the new instance set over failures from old instance sets.
	if len(conditions) > 0 {
		return conditions
	}

	for i := range allISs {
		is := allISs[i]
		if is == nil {
			continue
		}

		for _, c := range is.Status.Conditions {
			if c.Type != v1alpha1.InstanceSetReplicaFailure {
				continue
			}
			conditions = append(conditions, InstanceSetToInstanceDeploymentCondition(c))
		}
	}
	return conditions
}

// // used for unit testing
// var nowFn = func() time.Time { return time.Now() }

// requeueStuckDeployment checks whether the provided deployment needs to be synced for a progress
// check. It returns the time after the deployment will be requeued for the progress check, 0 if it
// will be requeued now, or -1 if it does not need to be requeued.
func (dc *controller) requeueStuckInstanceDeployment(d *v1alpha1.InstanceDeployment, newStatus v1alpha1.InstanceDeploymentStatus) time.Duration {
	currentCond := GetInstanceDeploymentCondition(d.Status, v1alpha1.InstanceDeploymentProgressing)
	// Can't estimate progress if there is no deadline in the spec or progressing condition in the current status.
	if d.Spec.ProgressDeadlineSeconds == nil || currentCond == nil {
		return time.Duration(-1)
	}
	// No need to estimate progress if the rollout is complete or already timed out.
	if InstanceDeploymentComplete(d, &newStatus) || currentCond.Reason == TimedOutReason {
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
		glog.V(4).Infof("Queueing up instance deployment %q for a progress check now", d.Name)
		dc.enqueueRateLimited(d)
		return time.Duration(0)
	}
	glog.V(4).Infof("Queueing up instance deployment %q for a progress check after %ds", d.Name, int(after.Seconds()))
	// Add a second to avoid milliseconds skew in AddAfter.
	// See https://github.com/kubernetes/kubernetes/issues/39785#issuecomment-279959133 for more info.
	dc.enqueueAfter(d, after+time.Second)
	return after
}
