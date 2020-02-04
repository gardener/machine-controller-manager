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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/rolling.go

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"fmt"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/integer"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/klog"
)

var (
	maxRetryDeadline      = 1 * time.Minute
	conflictRetryInterval = 5 * time.Second
)

// rolloutRolling implements the logic for rolling a new machine set.
func (dc *controller) rolloutRolling(d *v1alpha1.MachineDeployment, isList []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) error {
	newIS, oldISs, err := dc.getAllMachineSetsAndSyncRevision(d, isList, machineMap, true)
	if err != nil {
		return err
	}
	allISs := append(oldISs, newIS)

	err = dc.taintNodesBackingMachineSets(
		oldISs, &v1.Taint{
			Key:    PreferNoScheduleKey,
			Value:  "True",
			Effect: "PreferNoSchedule",
		},
	)
	if err != nil {
		klog.Warningf("Failed to add %s on all nodes. Error: %s", PreferNoScheduleKey, err)
	}

	// Scale up, if we can.
	scaledUp, err := dc.reconcileNewMachineSet(allISs, newIS, d)
	if err != nil {
		return err
	}
	if scaledUp {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// Scale down, if we can.
	scaledDown, err := dc.reconcileOldMachineSets(allISs, FilterActiveMachineSets(oldISs), newIS, d)
	if err != nil {
		return err
	}
	if scaledDown {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	if MachineDeploymentComplete(d, &d.Status) {
		if err := dc.cleanupMachineDeployment(oldISs, d); err != nil {
			return err
		}
	}

	// Sync deployment status
	return dc.syncRolloutStatus(allISs, newIS, d)
}

func (dc *controller) reconcileNewMachineSet(allISs []*v1alpha1.MachineSet, newIS *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	if (newIS.Spec.Replicas) == (deployment.Spec.Replicas) {
		// Scaling not required.
		return false, nil
	}
	if (newIS.Spec.Replicas) > (deployment.Spec.Replicas) {
		// Scale down.
		scaled, _, err := dc.scaleMachineSetAndRecordEvent(newIS, (deployment.Spec.Replicas), deployment)
		return scaled, err
	}
	newReplicasCount, err := NewISNewReplicas(deployment, allISs, newIS)
	if err != nil {
		return false, err
	}
	scaled, _, err := dc.scaleMachineSetAndRecordEvent(newIS, newReplicasCount, deployment)
	return scaled, err
}

func (dc *controller) reconcileOldMachineSets(allISs []*v1alpha1.MachineSet, oldISs []*v1alpha1.MachineSet, newIS *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	oldMachinesCount := GetReplicaCountForMachineSets(oldISs)
	if oldMachinesCount == 0 {
		// Can't scale down further
		return false, nil
	}

	allMachinesCount := GetReplicaCountForMachineSets(allISs)
	klog.V(4).Infof("New machine set %s has %d available machines.", newIS.Name, newIS.Status.AvailableReplicas)
	maxUnavailable := MaxUnavailable(*deployment)

	// Check if we can scale down. We can scale down in the following 2 cases:
	// * Some old machine sets have unhealthy replicas, we could safely scale down those unhealthy replicas since that won't further
	//  increase unavailability.
	// * New machine set has scaled up and it's replicas becomes ready, then we can scale down old machine sets in a further step.
	//
	// maxScaledDown := allmachinesCount - minAvailable - newReplicaSetmachinesUnavailable
	// take into account not only maxUnavailable and any surge machines that have been created, but also unavailable machines from
	// the newRS, so that the unavailable machines from the newRS would not make us scale down old machine sets in a further
	// step(that will increase unavailability).
	//
	// Concrete example:
	//
	// * 10 replicas
	// * 2 maxUnavailable (absolute number, not percent)
	// * 3 maxSurge (absolute number, not percent)
	//
	// case 1:
	// * Deployment is updated, newRS is created with 3 replicas, oldRS is scaled down to 8, and newRS is scaled up to 5.
	// * The new machine set machines crashloop and never become available.
	// * allmachinesCount is 13. minAvailable is 8. newRSmachinesUnavailable is 5.
	// * A node fails and causes one of the oldRS machines to become unavailable. However, 13 - 8 - 5 = 0, so the oldRS won't be scaled down.
	// * The user notices the crashloop and does kubectl rollout undo to rollback.
	// * newRSmachinesUnavailable is 1, since we rolled back to the good machine set, so maxScaledDown = 13 - 8 - 1 = 4. 4 of the crashlooping machines will be scaled down.
	// * The total number of machines will then be 9 and the newRS can be scaled up to 10.
	//
	// case 2:
	// Same example, but pushing a new machine template instead of rolling back (aka "roll over"):
	// * The new machine set created must start with 0 replicas because allmachinesCount is already at 13.
	// * However, newRSmachinesUnavailable would also be 0, so the 2 old machine sets could be scaled down by 5 (13 - 8 - 0), which would then
	// allow the new machine set to be scaled up by 5.
	minAvailable := (deployment.Spec.Replicas) - maxUnavailable
	newISUnavailableMachineCount := (newIS.Spec.Replicas) - newIS.Status.AvailableReplicas
	maxScaledDown := allMachinesCount - minAvailable - newISUnavailableMachineCount
	if maxScaledDown <= 0 {
		return false, nil
	}

	// Clean up unhealthy replicas first, otherwise unhealthy replicas will block deployment
	// and cause timeout. See https://github.com/kubernetes/kubernetes/issues/16737
	oldISs, cleanupCount, err := dc.cleanupUnhealthyReplicas(oldISs, deployment, maxScaledDown)
	if err != nil {
		return false, nil
	}
	klog.V(4).Infof("Cleaned up unhealthy replicas from old ISes by %d", cleanupCount)

	// Scale down old machine sets, need check maxUnavailable to ensure we can scale down
	allISs = append(oldISs, newIS)
	scaledDownCount, err := dc.scaleDownOldMachineSetsForRollingUpdate(allISs, oldISs, deployment)
	if err != nil {
		return false, nil
	}
	klog.V(4).Infof("Scaled down old ISes of deployment %s by %d", deployment.Name, scaledDownCount)

	totalScaledDown := cleanupCount + scaledDownCount
	return totalScaledDown > 0, nil
}

// cleanupUnhealthyReplicas will scale down old machine sets with unhealthy replicas, so that all unhealthy replicas will be deleted.
func (dc *controller) cleanupUnhealthyReplicas(oldISs []*v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment, maxCleanupCount int32) ([]*v1alpha1.MachineSet, int32, error) {
	sort.Sort(MachineSetsByCreationTimestamp(oldISs))
	// Safely scale down all old machine sets with unhealthy replicas. machine set will sort the machines in the order
	// such that not-ready < ready, unscheduled < scheduled, and pending < running. This ensures that unhealthy replicas will
	// been deleted first and won't increase unavailability.
	totalScaledDown := int32(0)
	for i, targetIS := range oldISs {
		if totalScaledDown >= maxCleanupCount {
			break
		}
		if (targetIS.Spec.Replicas) == 0 {
			// cannot scale down this machine set.
			continue
		}
		klog.V(4).Infof("Found %d available machine in old IS %s", targetIS.Status.AvailableReplicas, targetIS.Name)
		if (targetIS.Spec.Replicas) == targetIS.Status.AvailableReplicas {
			// no unhealthy replicas found, no scaling required.
			continue
		}

		scaledDownCount := int32(integer.IntMin(int(maxCleanupCount-totalScaledDown), int((targetIS.Spec.Replicas)-targetIS.Status.AvailableReplicas)))
		newReplicasCount := (targetIS.Spec.Replicas) - scaledDownCount
		if newReplicasCount > (targetIS.Spec.Replicas) {
			return nil, 0, fmt.Errorf("when cleaning up unhealthy replicas, got invalid request to scale down %s %d -> %d", targetIS.Name, (targetIS.Spec.Replicas), newReplicasCount)
		}
		_, updatedOldIS, err := dc.scaleMachineSetAndRecordEvent(targetIS, newReplicasCount, deployment)
		if err != nil {
			return nil, totalScaledDown, err
		}
		totalScaledDown += scaledDownCount
		oldISs[i] = updatedOldIS
	}
	return oldISs, totalScaledDown, nil
}

// scaleDownOldReplicaSetsForRollingUpdate scales down old machine sets when deployment strategy is "RollingUpdate".
// Need check maxUnavailable to ensure availability
func (dc *controller) scaleDownOldMachineSetsForRollingUpdate(allISs []*v1alpha1.MachineSet, oldISs []*v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (int32, error) {
	maxUnavailable := MaxUnavailable(*deployment)

	// Check if we can scale down.
	minAvailable := (deployment.Spec.Replicas) - maxUnavailable
	// Find the number of available machines.
	availableMachineCount := GetAvailableReplicaCountForMachineSets(allISs)
	if availableMachineCount <= minAvailable {
		// Cannot scale down.
		return 0, nil
	}
	klog.V(4).Infof("Found %d available machines in deployment %s, scaling down old ISes", availableMachineCount, deployment.Name)

	sort.Sort(MachineSetsByCreationTimestamp(oldISs))

	totalScaledDown := int32(0)
	totalScaleDownCount := availableMachineCount - minAvailable
	for _, targetIS := range oldISs {
		if totalScaledDown >= totalScaleDownCount {
			// No further scaling required.
			break
		}
		if (targetIS.Spec.Replicas) == 0 {
			// cannot scale down this ReplicaSet.
			continue
		}
		// Scale down.
		scaleDownCount := int32(integer.IntMin(int((targetIS.Spec.Replicas)), int(totalScaleDownCount-totalScaledDown)))
		newReplicasCount := (targetIS.Spec.Replicas) - scaleDownCount
		if newReplicasCount > (targetIS.Spec.Replicas) {
			return 0, fmt.Errorf("when scaling down old IS, got invalid request to scale down %s %d -> %d", targetIS.Name, (targetIS.Spec.Replicas), newReplicasCount)
		}
		_, _, err := dc.scaleMachineSetAndRecordEvent(targetIS, newReplicasCount, deployment)
		if err != nil {
			return totalScaledDown, err
		}

		totalScaledDown += scaleDownCount
	}

	return totalScaledDown, nil
}

// taintNodesBackingMachineSets taints all nodes backing the machineSets
func (dc *controller) taintNodesBackingMachineSets(MachineSets []*v1alpha1.MachineSet, taint *v1.Taint) error {

	for _, machineSet := range MachineSets {

		if _, exists := machineSet.Annotations[taint.Key]; exists {
			// Taint exists, hence just continue
			continue
		}

		klog.V(3).Infof("Trying to taint MachineSet object %q with %s to avoid scheduling of pods", machineSet.Name, taint.Key)
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

		// Iterate through all machines and place the PreferNoSchedule taint
		// to avoid scheduling on older machines
		for _, machine := range filteredMachines {
			if machine.Status.Node != "" {
				err = AddOrUpdateTaintOnNode(
					dc.targetCoreClient,
					machine.Status.Node,
					taint,
				)
				if err != nil {
					klog.Warningf("Node tainting failed for node: %s, %s", machine.Status.Node, err)
				}
			}
		}

		retryDeadline := time.Now().Add(maxRetryDeadline)
		for {
			machineSet, err = dc.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
			if err != nil && time.Now().Before(retryDeadline) {
				klog.Warningf("Unable to fetch MachineSet object %s, Error: %+v", machineSet.Name, err)
				time.Sleep(conflictRetryInterval)
				continue
			} else if err != nil {
				// Timeout occurred
				klog.Errorf("Timeout occurred: Unable to fetch MachineSet object %s, Error: %+v", machineSet.Name, err)
				return err
			}

			msCopy := machineSet.DeepCopy()
			if msCopy.Annotations == nil {
				msCopy.Annotations = make(map[string]string, 0)
			}
			msCopy.Annotations[taint.Key] = "True"

			_, err = dc.controlMachineClient.MachineSets(msCopy.Namespace).Update(msCopy)
			if err != nil && time.Now().Before(retryDeadline) {
				klog.Warningf("Unable to update MachineSet object %s, Error: %+v", machineSet.Name, err)
				time.Sleep(conflictRetryInterval)
				continue
			} else if err != nil {
				// Timeout occurred
				klog.Errorf("Timeout occurred: Unable to update MachineSet object %s, Error: %+v", machineSet.Name, err)
				return err
			}

			// Break out of loop when update succeeds
			break
		}
		klog.V(2).Infof("Tainted MachineSet object %q with %s to avoid scheduling of pods", machineSet.Name, taint.Key)
	}

	return nil
}
