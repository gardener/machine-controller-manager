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

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"fmt"
	"sort"

	"github.com/golang/glog"
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/integer"
)

// rolloutRolling implements the logic for rolling a new instance set.
func (dc *controller) rolloutRolling(d *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet, instanceMap map[types.UID]*v1alpha1.InstanceList) error {
	newIS, oldISs, err := dc.getAllInstanceSetsAndSyncRevision(d, isList, instanceMap, true)
	if err != nil {
		return err
	}
	allISs := append(oldISs, newIS)

	// Scale up, if we can.
	scaledUp, err := dc.reconcileNewInstanceSet(allISs, newIS, d)
	if err != nil {
		return err
	}
	if scaledUp {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// Scale down, if we can.
	scaledDown, err := dc.reconcileOldInstanceSets(allISs, FilterActiveInstanceSets(oldISs), newIS, d)
	if err != nil {
		return err
	}
	if scaledDown {
		// Update DeploymentStatus
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	if InstanceDeploymentComplete(d, &d.Status) {
		if err := dc.cleanupInstanceDeployment(oldISs, d); err != nil {
			return err
		}
	}

	// Sync deployment status
	return dc.syncRolloutStatus(allISs, newIS, d)
}

func (dc *controller) reconcileNewInstanceSet(allISs []*v1alpha1.InstanceSet, newIS *v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment) (bool, error) {
	if (newIS.Spec.Replicas) == (deployment.Spec.Replicas) {
		// Scaling not required.
		return false, nil
	}
	if (newIS.Spec.Replicas) > (deployment.Spec.Replicas) {
		// Scale down.
		scaled, _, err := dc.scaleInstanceSetAndRecordEvent(newIS, (deployment.Spec.Replicas), deployment)
		return scaled, err
	}
	newReplicasCount, err := NewISNewReplicas(deployment, allISs, newIS)
	if err != nil {
		return false, err
	}
	scaled, _, err := dc.scaleInstanceSetAndRecordEvent(newIS, newReplicasCount, deployment)
	return scaled, err
}

func (dc *controller) reconcileOldInstanceSets(allISs []*v1alpha1.InstanceSet, oldISs []*v1alpha1.InstanceSet, newIS *v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment) (bool, error) {
	oldInstancesCount := GetReplicaCountForInstanceSets(oldISs)
	if oldInstancesCount == 0 {
		// Can't scale down further
		return false, nil
	}

	allInstancesCount := GetReplicaCountForInstanceSets(allISs)
	glog.V(4).Infof("New instance set %s has %d available instances.", newIS.Name, newIS.Status.AvailableReplicas)
	maxUnavailable := MaxUnavailable(*deployment)

	// Check if we can scale down. We can scale down in the following 2 cases:
	// * Some old instance sets have unhealthy replicas, we could safely scale down those unhealthy replicas since that won't further
	//  increase unavailability.
	// * New instance set has scaled up and it's replicas becomes ready, then we can scale down old instance sets in a further step.
	//
	// maxScaledDown := allinstancesCount - minAvailable - newReplicaSetinstancesUnavailable
	// take into account not only maxUnavailable and any surge instances that have been created, but also unavailable instances from
	// the newRS, so that the unavailable instances from the newRS would not make us scale down old instance sets in a further
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
	// * The new instance set instances crashloop and never become available.
	// * allinstancesCount is 13. minAvailable is 8. newRSinstancesUnavailable is 5.
	// * A node fails and causes one of the oldRS instances to become unavailable. However, 13 - 8 - 5 = 0, so the oldRS won't be scaled down.
	// * The user notices the crashloop and does kubectl rollout undo to rollback.
	// * newRSinstancesUnavailable is 1, since we rolled back to the good instance set, so maxScaledDown = 13 - 8 - 1 = 4. 4 of the crashlooping instances will be scaled down.
	// * The total number of instances will then be 9 and the newRS can be scaled up to 10.
	//
	// case 2:
	// Same example, but pushing a new instance template instead of rolling back (aka "roll over"):
	// * The new instance set created must start with 0 replicas because allinstancesCount is already at 13.
	// * However, newRSinstancesUnavailable would also be 0, so the 2 old instance sets could be scaled down by 5 (13 - 8 - 0), which would then
	// allow the new instance set to be scaled up by 5.
	minAvailable := (deployment.Spec.Replicas) - maxUnavailable
	newISUnavailableInstanceCount := (newIS.Spec.Replicas) - newIS.Status.AvailableReplicas
	maxScaledDown := allInstancesCount - minAvailable - newISUnavailableInstanceCount
	if maxScaledDown <= 0 {
		return false, nil
	}

	// Clean up unhealthy replicas first, otherwise unhealthy replicas will block deployment
	// and cause timeout. See https://github.com/kubernetes/kubernetes/issues/16737
	oldISs, cleanupCount, err := dc.cleanupUnhealthyReplicas(oldISs, deployment, maxScaledDown)
	if err != nil {
		return false, nil
	}
	glog.V(4).Infof("Cleaned up unhealthy replicas from old ISes by %d", cleanupCount)

	// Scale down old instance sets, need check maxUnavailable to ensure we can scale down
	allISs = append(oldISs, newIS)
	scaledDownCount, err := dc.scaleDownOldInstanceSetsForRollingUpdate(allISs, oldISs, deployment)
	if err != nil {
		return false, nil
	}
	glog.V(4).Infof("Scaled down old ISes of deployment %s by %d", deployment.Name, scaledDownCount)

	totalScaledDown := cleanupCount + scaledDownCount
	return totalScaledDown > 0, nil
}

// cleanupUnhealthyReplicas will scale down old instance sets with unhealthy replicas, so that all unhealthy replicas will be deleted.
func (dc *controller) cleanupUnhealthyReplicas(oldISs []*v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment, maxCleanupCount int32) ([]*v1alpha1.InstanceSet, int32, error) {
	sort.Sort(InstanceSetsByCreationTimestamp(oldISs))
	// Safely scale down all old instance sets with unhealthy replicas. instance set will sort the instances in the order
	// such that not-ready < ready, unscheduled < scheduled, and pending < running. This ensures that unhealthy replicas will
	// been deleted first and won't increase unavailability.
	totalScaledDown := int32(0)
	for i, targetIS := range oldISs {
		if totalScaledDown >= maxCleanupCount {
			break
		}
		if (targetIS.Spec.Replicas) == 0 {
			// cannot scale down this instance set.
			continue
		}
		glog.V(4).Infof("Found %d available instance in old IS %s", targetIS.Status.AvailableReplicas, targetIS.Name)
		if (targetIS.Spec.Replicas) == targetIS.Status.AvailableReplicas {
			// no unhealthy replicas found, no scaling required.
			continue
		}

		scaledDownCount := int32(integer.IntMin(int(maxCleanupCount-totalScaledDown), int((targetIS.Spec.Replicas)-targetIS.Status.AvailableReplicas)))
		newReplicasCount := (targetIS.Spec.Replicas) - scaledDownCount
		if newReplicasCount > (targetIS.Spec.Replicas) {
			return nil, 0, fmt.Errorf("when cleaning up unhealthy replicas, got invalid request to scale down %s %d -> %d", targetIS.Name, (targetIS.Spec.Replicas), newReplicasCount)
		}
		_, updatedOldIS, err := dc.scaleInstanceSetAndRecordEvent(targetIS, newReplicasCount, deployment)
		if err != nil {
			return nil, totalScaledDown, err
		}
		totalScaledDown += scaledDownCount
		oldISs[i] = updatedOldIS
	}
	return oldISs, totalScaledDown, nil
}

// scaleDownOldReplicaSetsForRollingUpdate scales down old instance sets when deployment strategy is "RollingUpdate".
// Need check maxUnavailable to ensure availability
func (dc *controller) scaleDownOldInstanceSetsForRollingUpdate(allISs []*v1alpha1.InstanceSet, oldISs []*v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment) (int32, error) {
	maxUnavailable := MaxUnavailable(*deployment)

	// Check if we can scale down.
	minAvailable := (deployment.Spec.Replicas) - maxUnavailable
	// Find the number of available instances.
	availableInstanceCount := GetAvailableReplicaCountForInstanceSets(allISs)
	if availableInstanceCount <= minAvailable {
		// Cannot scale down.
		return 0, nil
	}
	glog.V(4).Infof("Found %d available instances in deployment %s, scaling down old ISes", availableInstanceCount, deployment.Name)

	sort.Sort(InstanceSetsByCreationTimestamp(oldISs))

	totalScaledDown := int32(0)
	totalScaleDownCount := availableInstanceCount - minAvailable
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
		_, _, err := dc.scaleInstanceSetAndRecordEvent(targetIS, newReplicasCount, deployment)
		if err != nil {
			return totalScaledDown, err
		}

		totalScaledDown += scaleDownCount
	}

	return totalScaledDown, nil
}
