/*
Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.

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

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"
)

const (
	// OverShootingReplicaCount freeze reason when replica count overshoots
	OverShootingReplicaCount = "OverShootingReplicaCount"
	// MachineDeploymentStateSync freeze reason when machineDeployment was found with inconsistent state
	MachineDeploymentStateSync = "MachineDeploymentStateSync"
	// TimeoutOccurred freeze reason when machineSet timeout occurs
	TimeoutOccurred = "MachineSetTimeoutOccurred"
	// UnfreezeAnnotation indicates the controllers to unfreeze this object
	UnfreezeAnnotation = "safety.machine.sapcloud.io/unfreeze"
)

// reconcileClusterMachineSafetyOrphanVMs checks for any orphan VMs and deletes them
func (c *controller) reconcileClusterMachineSafetyOrphanVMs(key string) error {
	reSyncAfter := c.safetyOptions.MachineSafetyOrphanVMsPeriod.Duration
	defer c.machineSafetyOrphanVMsQueue.AddAfter("", reSyncAfter)

	klog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: Start")
	defer klog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: End, reSync-Period: %v", reSyncAfter)

	retry, err := c.checkMachineClasses()
	if err != nil {
		klog.Errorf("reconcileClusterMachineSafetyOrphanVMs: Error occurred while checking for orphan VMs: %s", err)
		if retry {
			return err
		}
	}

	return nil
}

// reconcileClusterMachineSafetyAPIServer checks control and target clusters
// and checks if their APIServer's are reachable
// If they are not reachable, they set a machineControllerFreeze flag
func (c *controller) reconcileClusterMachineSafetyAPIServer(key string) error {
	statusCheckTimeout := c.safetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration
	statusCheckPeriod := c.safetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration

	klog.V(4).Infof("reconcileClusterMachineSafetyAPIServer: Start")
	defer klog.V(4).Infof("reconcileClusterMachineSafetyAPIServer: Stop")

	if c.safetyOptions.MachineControllerFrozen {
		// MachineController is frozen
		if c.isAPIServerUp() {
			// APIServer is up now, hence we need reset all machine health checks (to avoid unwanted freezes) and unfreeze
			machines, err := c.machineLister.List(labels.Everything())
			if err != nil {
				klog.Error("SafetyController: Unable to LIST machines. Error:", err)
				return err
			}
			for _, machine := range machines {
				if machine.Status.CurrentStatus.Phase == v1alpha1.MachineUnknown {
					machine, err := c.controlMachineClient.Machines(c.namespace).Get(machine.Name, metav1.GetOptions{})
					if err != nil {
						klog.Error("SafetyController: Unable to GET machines. Error:", err)
						return err
					}

					machine.Status.CurrentStatus = v1alpha1.CurrentStatus{
						Phase:          v1alpha1.MachineRunning,
						TimeoutActive:  false,
						LastUpdateTime: metav1.Now(),
					}
					machine.Status.LastOperation = v1alpha1.LastOperation{
						Description:    "Machine Health Timeout was reset due to APIServer being unreachable",
						LastUpdateTime: metav1.Now(),
						State:          v1alpha1.MachineStateSuccessful,
						Type:           v1alpha1.MachineOperationHealthCheck,
					}
					_, err = c.controlMachineClient.Machines(c.namespace).UpdateStatus(machine)
					if err != nil {
						klog.Error("SafetyController: Unable to UPDATE machine/status. Error:", err)
						return err
					}

					klog.V(2).Info("SafetyController: Reinitializing machine health check for ", machine.Name)
				}

				// En-queue after 30 seconds, to ensure all machine states are reconciled
				c.enqueueMachineAfter(machine, 30*time.Second)
			}

			c.safetyOptions.MachineControllerFrozen = false
			c.safetyOptions.APIserverInactiveStartTime = time.Time{}
			klog.V(2).Infof("SafetyController: UnFreezing Machine Controller")
		}
	} else {
		// MachineController is not frozen
		if !c.isAPIServerUp() {
			// If APIServer is not up
			if c.safetyOptions.APIserverInactiveStartTime.Equal(time.Time{}) {
				// If timeout has not started
				c.safetyOptions.APIserverInactiveStartTime = time.Now()
			}
			if time.Now().Sub(c.safetyOptions.APIserverInactiveStartTime) > statusCheckTimeout {
				// If APIServer has been down for more than statusCheckTimeout
				c.safetyOptions.MachineControllerFrozen = true
				klog.V(2).Infof("SafetyController: Freezing Machine Controller")
			}

			// Re-enqueue the safety check more often if APIServer is not active and is not frozen yet
			defer c.machineSafetyAPIServerQueue.AddAfter("", statusCheckTimeout/5)
			return nil
		}
	}

	defer c.machineSafetyAPIServerQueue.AddAfter("", statusCheckPeriod)
	return nil
}

// isAPIServerUp returns true if APIServers are up
// Both control and target APIServers
func (c *controller) isAPIServerUp() bool {
	// Dummy get call to check if control APIServer is reachable
	_, err := c.controlMachineClient.Machines(c.namespace).Get("dummy_name", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Get returns an error other than object not found = Assume APIServer is not reachable
		klog.Error("SafetyController: Unable to GET on machine objects ", err)
		return false
	}

	// Dummy get call to check if target APIServer is reachable
	_, err = c.targetCoreClient.CoreV1().Nodes().Get("dummy_name", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Get returns an error other than object not found = Assume APIServer is not reachable
		klog.Error("SafetyController: Unable to GET on node objects ", err)
		return false
	}

	return true
}

// checkCommonMachineClass checks for orphan VMs in MachinesClasses
func (c *controller) checkMachineClasses() (machineutils.Retry, error) {
	MachineClasses, err := c.machineClassLister.List(labels.Everything())
	if err != nil {
		klog.Error("Safety-Net: Error getting machineClasses")
		return machineutils.DoNotRetryOp, err
	}

	for _, machineClass := range MachineClasses {
		retry, err := c.checkMachineClass(
			machineClass,
			machineClass.SecretRef,
		)
		if err != nil {
			return retry, err
		}
	}

	return machineutils.DoNotRetryOp, nil
}

// checkMachineClass checks a particular machineClass for orphan instances
func (c *controller) checkMachineClass(
	machineClass *v1alpha1.MachineClass,
	secretRef *corev1.SecretReference) (machineutils.Retry, error) {

	// Get secret
	secret, err := c.getSecret(secretRef, machineClass.Name)
	if err != nil || secret == nil {
		klog.Errorf("SafetyController: Secret reference not found for MachineClass: %q", machineClass.Name)
		return machineutils.DoNotRetryOp, err
	}

	listMachineResponse, err := c.driver.ListMachines(context.TODO(), &driver.ListMachinesRequest{
		MachineClass: machineClass,
		Secret:       secret,
	})
	if err != nil {
		klog.Errorf("SafetyController: Failed to LIST VMs at provider. Error: %s", err)
		return machineutils.RetryOp, err
	}

	// Making sure that its not a VM just being created, machine object not yet updated at API server
	if len(listMachineResponse.MachineList) > 1 {
		stopCh := make(chan struct{})
		defer close(stopCh)

		if !cache.WaitForCacheSync(stopCh, c.machineSynced) {
			klog.Errorf("SafetyController: Timed out waiting for caches to sync. Error: %s", err)
			return machineutils.RetryOp, err
		}
	}

	for machineID, machineName := range listMachineResponse.MachineList {
		machine, err := c.machineLister.Machines(c.namespace).Get(machineName)

		if err != nil && !apierrors.IsNotFound(err) {
			// Any other types of errors
			klog.Errorf("SafetyController: Error while trying to GET machines. Error: %s", err)
		} else if err != nil || machine.Spec.ProviderID != machineID {

			// If machine exists and machine object is still been processed by the machine controller
			if err == nil &&
				machine.Status.CurrentStatus.Phase == "" {
				klog.V(3).Infof("SafetyController: Machine object %q is being processed by machine controller, hence skipping", machine.Name)
				continue
			}

			// Creating a dummy machine object to create deleteMachineRequest
			machine = &v1alpha1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: machineName,
				},
				Spec: v1alpha1.MachineSpec{
					ProviderID: machineID,
				},
			}

			_, err := c.driver.DeleteMachine(context.TODO(), &driver.DeleteMachineRequest{
				Machine:      machine,
				MachineClass: machineClass,
				Secret:       secret,
			})
			if err != nil {
				klog.Errorf("SafetyController: Error while trying to DELETE VM on CP - %s. Shall retry in next safety controller sync.", err)
			} else {
				klog.V(2).Infof("SafetyController: Orphan VM found and terminated VM: %s, %s", machineName, machineID)
			}
		}
	}
	return machineutils.DoNotRetryOp, nil
}

// deleteMachineToSafety enqueues into machineSafetyQueue when a new machine is deleted
func (c *controller) deleteMachineToSafety(obj interface{}) {
	machine := obj.(*v1alpha1.Machine)
	c.enqueueMachineSafetyOrphanVMsKey(machine)
}

// enqueueMachineSafetyOrphanVMsKey enqueues into machineSafetyOrphanVMsQueue
func (c *controller) enqueueMachineSafetyOrphanVMsKey(obj interface{}) {
	c.machineSafetyOrphanVMsQueue.Add("")
}
