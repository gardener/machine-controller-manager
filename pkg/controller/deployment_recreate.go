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

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
)

// rolloutRecreate implements the logic for recreating a machine set.
func (dc *controller) rolloutRecreate(d *v1alpha1.MachineDeployment, isList []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) error {
	// Don't create a new RS if not already existed, so that we avoid scaling up before scaling down.
	newIS, oldISs, err := dc.getAllMachineSetsAndSyncRevision(d, isList, machineMap, false)
	if err != nil {
		return err
	}
	allISs := append(oldISs, newIS)
	activeOldISs := FilterActiveMachineSets(oldISs)

	// scale down old machine sets.
	scaledDown, err := dc.scaleDownOldMachineSetsForRecreate(activeOldISs, d)
	if err != nil {
		return err
	}
	if scaledDown {
		// Update DeploymentStatus.
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// Do not process a deployment when it has old machines running.
	if oldMachinesRunning(newIS, oldISs, machineMap) {
		return dc.syncRolloutStatus(allISs, newIS, d)
	}

	// If we need to create a new RS, create it now.
	if newIS == nil {
		newIS, oldISs, err = dc.getAllMachineSetsAndSyncRevision(d, isList, machineMap, true)
		if err != nil {
			return err
		}
		allISs = append(oldISs, newIS)
	}

	// scale up new machine set.
	if _, err := dc.scaleUpNewMachineSetForRecreate(newIS, d); err != nil {
		return err
	}

	if MachineDeploymentComplete(d, &d.Status) {
		if err := dc.cleanupMachineDeployment(oldISs, d); err != nil {
			return err
		}
	}

	// Sync deployment status.
	return dc.syncRolloutStatus(allISs, newIS, d)
}

// scaleDownOldMachineSetsForRecreate scales down old machine sets when deployment strategy is "Recreate".
func (dc *controller) scaleDownOldMachineSetsForRecreate(oldISs []*v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	scaled := false
	for i := range oldISs {
		is := oldISs[i]
		// Scaling not required.
		if (is.Spec.Replicas) == 0 {
			continue
		}
		scaledIS, updatedIS, err := dc.scaleMachineSetAndRecordEvent(is, 0, deployment)
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

// oldmachinesRunning returns whether there are old machines running or any of the old MachineSets thinks that it runs machines.
func oldMachinesRunning(newIS *v1alpha1.MachineSet, oldISs []*v1alpha1.MachineSet, machineMap map[types.UID]*v1alpha1.MachineList) bool {
	if oldMachines := GetActualReplicaCountForMachineSets(oldISs); oldMachines > 0 {
		return true
	}
	for isUID, machineList := range machineMap {
		// If the machines belong to the new MachineSet, ignore.
		if newIS != nil && newIS.UID == isUID {
			continue
		}
		if len(machineList.Items) > 0 {
			return true
		}
	}
	return false
}

// scaleUpNewMachineSetForRecreate scales up new machine set when deployment strategy is "Recreate".
func (dc *controller) scaleUpNewMachineSetForRecreate(newIS *v1alpha1.MachineSet, deployment *v1alpha1.MachineDeployment) (bool, error) {
	scaled, _, err := dc.scaleMachineSetAndRecordEvent(newIS, (deployment.Spec.Replicas), deployment)
	return scaled, err
}
