/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
)

/*
	SECTION
	Update machine object
*/

func (c *controller) updateMachineStatus(
	ctx context.Context,
	machine *v1alpha1.Machine,
	lastOperation v1alpha1.LastOperation,
	currentStatus v1alpha1.CurrentStatus,
) (*v1alpha1.Machine, error) {
	// Get the latest version of the machine so that we can avoid conflicts
	latestMachine, err := c.controlMachineClient.Machines(machine.Namespace).Get(ctx, machine.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	clone := latestMachine.DeepCopy()

	clone.Status.LastOperation = lastOperation
	clone.Status.CurrentStatus = currentStatus
	if isMachineStatusEqual(clone.Status, machine.Status) {
		klog.V(3).Infof("Not updating the status of the machine object %q , as it is already same", clone.Name)
		return machine, nil
	}

	clone, err = c.controlMachineClient.Machines(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		// Keep retrying until update goes through
		klog.V(3).Infof("Warning: Updated failed, retrying, error: %q", err)
		return c.updateMachineStatus(ctx, machine, lastOperation, currentStatus)
	}
	return clone, nil
}

// isMachineStatusEqual checks if the status of 2 machines is similar or not.
func isMachineStatusEqual(s1, s2 v1alpha1.MachineStatus) bool {
	tolerateTimeDiff := 30 * time.Minute
	s1Copy, s2Copy := s1.DeepCopy(), s2.DeepCopy()
	s1Copy.LastOperation.Description, s2Copy.LastOperation.Description = "", ""

	if (s1Copy.LastOperation.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) || (s2Copy.LastOperation.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) {
		return false
	}
	s1Copy.LastOperation.LastUpdateTime, s2Copy.LastOperation.LastUpdateTime = metav1.Time{}, metav1.Time{}

	if (s1Copy.CurrentStatus.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) || (s2Copy.CurrentStatus.LastUpdateTime.Time.Before(time.Now().Add(tolerateTimeDiff * -1))) {
		return false
	}
	s1Copy.CurrentStatus.LastUpdateTime, s2Copy.CurrentStatus.LastUpdateTime = metav1.Time{}, metav1.Time{}

	return apiequality.Semantic.DeepEqual(s1Copy.LastOperation, s2Copy.LastOperation) && apiequality.Semantic.DeepEqual(s1Copy.CurrentStatus, s2Copy.CurrentStatus)
}
