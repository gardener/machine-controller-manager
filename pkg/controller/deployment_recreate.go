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
https://github.com/kubernetes/kubernetes/release-1.8/pkg/controller/deployment/recreate.go

Modifications Copyright 2017 The Gardener Authors.
*/

package controller

import (
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// rolloutRecreate implements the logic for recreating a instance set.
func (dc *controller) rolloutRecreate(d *v1alpha1.InstanceDeployment, isList []*v1alpha1.InstanceSet, instanceMap map[types.UID]*v1alpha1.InstanceList) error {
	// Don't create a new RS if not already existed, so that we avoid scaling up before scaling down.
	newIS, oldISs, err := dc.getAllInstanceSetsAndSyncRevision(d, isList, instanceMap, false)
	if err != nil {
		return err
	}
	allISs := append(oldISs, newIS)
	activeOldISs := FilterActiveInstanceSets(oldISs)

	// scale down old instance sets.
	scaledDown, err := dc.scaleDownOldInstanceSetsForRecreate(activeOldISs, d)
	if err != nil {
		return err
	}
	if scaledDown {
		// Update DeploymentStatus.
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// Do not process a deployment when it has old instances running.
	if oldInstancesRunning(newIS, oldISs, instanceMap) {
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// If we need to create a new RS, create it now.
	if newIS == nil {
		newIS, oldISs, err = dc.getAllInstanceSetsAndSyncRevision(d, isList, instanceMap, true)
		if err != nil {
			return err
		}
		allISs = append(oldISs, newIS)
	}

	// scale up new instance set.
	if _, err := dc.scaleUpNewInstanceSetForRecreate(newIS, d); err != nil {
		return err
	}

	if InstanceDeploymentComplete(d, &d.Status) {
		if err := dc.cleanupInstanceDeployment(oldISs, d); err != nil {
			return err
		}
	}

	// Sync deployment status.
	return dc.syncRolloutStatus(allISs, newIS, d)
}

// scaleDownOldInstanceSetsForRecreate scales down old instance sets when deployment strategy is "Recreate".
func (dc *controller) scaleDownOldInstanceSetsForRecreate(oldISs []*v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment) (bool, error) {
	scaled := false
	for i := range oldISs {
		is := oldISs[i]
		// Scaling not required.
		if (is.Spec.Replicas) == 0 {
			continue
		}
		scaledIS, updatedIS, err := dc.scaleInstanceSetAndRecordEvent(is, 0, deployment)
		if err != nil {
			return false, err
		}
		if scaledIS {
			oldISs[i] = updatedIS
			scaled = true
		}
	}
	return scaled, nil
}

// oldinstancesRunning returns whether there are old instances running or any of the old InstanceSets thinks that it runs instances.
func oldInstancesRunning(newIS *v1alpha1.InstanceSet, oldISs []*v1alpha1.InstanceSet, instanceMap map[types.UID]*v1alpha1.InstanceList) bool {
	if oldInstances := GetActualReplicaCountForInstanceSets(oldISs); oldInstances > 0 {
		return true
	}
	for isUID, instanceList := range instanceMap {
		// If the instances belong to the new InstanceSet, ignore.
		if newIS != nil && newIS.UID == isUID {
			continue
		}
		if len(instanceList.Items) > 0 {
			return true
		}
	}
	return false
}

// scaleUpNewInstanceSetForRecreate scales up new instance set when deployment strategy is "Recreate".
func (dc *controller) scaleUpNewInstanceSetForRecreate(newIS *v1alpha1.InstanceSet, deployment *v1alpha1.InstanceDeployment) (bool, error) {
	scaled, _, err := dc.scaleInstanceSetAndRecordEvent(newIS, (deployment.Spec.Replicas), deployment)
	return scaled, err
}
