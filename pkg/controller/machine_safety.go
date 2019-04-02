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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/cache"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/driver"
	"github.com/golang/glog"
)

const (
	// OverShootingReplicaCount freeze reason when replica count overshoots
	OverShootingReplicaCount = "OverShootingReplicaCount"
	// TimeoutOccurred freeze reason when machineSet timeout occurs
	TimeoutOccurred = "MachineSetTimeoutOccurred"
	// LastReplicaUpdate contains the last timestamp when the
	// number of replicas was changed
	LastReplicaUpdate = "safety.machine.sapcloud.io/lastreplicaupdate"
)

// reconcileClusterMachineSafetyOrphanVMs checks for any orphan VMs and deletes them
func (c *controller) reconcileClusterMachineSafetyOrphanVMs(key string) error {
	reSyncAfter := c.safetyOptions.MachineSafetyOrphanVMsPeriod.Duration

	defer c.machineSafetyOrphanVMsQueue.AddAfter("", reSyncAfter)
	glog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: Start")
	c.checkVMObjects()
	glog.V(3).Infof("reconcileClusterMachineSafetyOrphanVMs: End, reSync-Period: %v", reSyncAfter)

	return nil
}

// reconcileClusterMachineSafetyOvershooting checks all machineSet/machineDeployment
// if the number of machine objects backing them is way beyond its desired replicas
func (c *controller) reconcileClusterMachineSafetyOvershooting(key string) error {
	reSyncAfter := c.safetyOptions.MachineSafetyOvershootingPeriod.Duration

	defer c.machineSafetyOvershootingQueue.AddAfter("", reSyncAfter)
	glog.V(3).Infof("reconcileClusterMachineSafetyOvershooting: Start")
	err := c.checkAndFreezeORUnfreezeMachineSets()
	glog.V(3).Infof("reconcileClusterMachineSafetyOvershooting: End, reSync-Period: %v", reSyncAfter)

	return err
}

// reconcileClusterMachineSafetyAPIServer checks control and target clusters
// and checks if their APIServer's are reachable
// If they are not reachable, they set a machineControllerFreeze flag
func (c *controller) reconcileClusterMachineSafetyAPIServer(key string) error {
	statusCheckTimeout := c.safetyOptions.MachineSafetyAPIServerStatusCheckTimeout.Duration
	statusCheckPeriod := c.safetyOptions.MachineSafetyAPIServerStatusCheckPeriod.Duration

	glog.V(3).Infof("reconcileClusterMachineSafetyAPIServer: Start")
	defer glog.V(3).Infof("reconcileClusterMachineSafetyAPIServer: Stop")

	if c.safetyOptions.MachineControllerFrozen {
		// MachineController is frozen
		if c.isAPIServerUp() {
			// APIServer is up now, hence we need reset all machine health checks (to avoid unwanted freezes) and unfreeze
			machines, err := c.machineLister.List(labels.Everything())
			if err != nil {
				glog.Warning(err)
				return err
			}
			for _, machine := range machines {
				if machine.Status.CurrentStatus.Phase == v1alpha1.MachineUnknown {
					machine, err := c.controlMachineClient.Machines(c.namespace).Get(machine.Name, metav1.GetOptions{})
					if err != nil {
						glog.Warning(err)
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
						glog.Warning(err)
						return err
					}

					glog.Info("reconcileClusterMachineSafetyAPIServer: Reinitializing machine health check for ", machine.Name)
				}

				// En-queue after 30 seconds, to ensure all machine states are reconciled
				c.enqueueMachineAfter(machine, 30*time.Second)
			}

			c.safetyOptions.MachineControllerFrozen = false
			c.safetyOptions.APIserverInactiveStartTime = time.Time{}
			glog.V(2).Infof("reconcileClusterMachineSafetyAPIServer: UnFreezing Machine Controller")
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
				glog.V(2).Infof("reconcileClusterMachineSafetyAPIServer: Freezing Machine Controller")
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
		glog.Warning("Unable to get on machine objects ", err)
		return false
	}

	// Dummy get call to check if target APIServer is reachable
	_, err = c.targetCoreClient.CoreV1().Nodes().Get("dummy_name", metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Get returns an error other than object not found = Assume APIServer is not reachable
		glog.Warning("Unable to get on node objects ", err)
		return false
	}

	return true
}

// checkAndFreezeORUnfreezeMachineSets freezes/unfreezes machineSets/machineDeployments
// which have much greater than desired number of replicas of machine objects
func (c *controller) checkAndFreezeORUnfreezeMachineSets() error {
	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		glog.Error("checkAndFreezeORUnfreezeMachineSets: Error getting machineSets - ", err)
		return err
	}

	for _, machineSet := range machineSets {

		filteredMachines, err := c.machineLister.List(labels.Everything())
		if err != nil {
			glog.Error("checkAndFreezeORUnfreezeMachineSets: Error getting machines - ", err)
			return err
		}
		fullyLabeledReplicasCount := int32(0)
		templateLabel := labels.Set(machineSet.Spec.Template.Labels).AsSelectorPreValidated()
		for _, machine := range filteredMachines {
			if templateLabel.Matches(labels.Set(machine.Labels)) &&
				len(machine.OwnerReferences) >= 1 {
				for i := range machine.OwnerReferences {
					if machine.OwnerReferences[i].Name == machineSet.Name {
						fullyLabeledReplicasCount++
					}
				}
			}
		}

		// Freeze machinesets when replica count exceeds by SafetyUP
		higherThreshold := 2*machineSet.Spec.Replicas + c.safetyOptions.SafetyUp
		// Unfreeze machineset when replica count reaches higherThreshold - SafetyDown
		lowerThreshold := higherThreshold - c.safetyOptions.SafetyDown

		machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
		if len(machineDeployments) >= 1 {
			machineDeployment := machineDeployments[0]
			if machineDeployment != nil {
				surge, err := intstrutil.GetValueFromIntOrPercent(
					machineDeployment.Spec.Strategy.RollingUpdate.MaxSurge,
					int(machineDeployment.Spec.Replicas),
					true,
				)
				if err != nil {
					glog.Error("checkAndFreezeORUnfreezeMachineSets: Error getting surge value - ", err)
					return err
				}

				higherThreshold = machineDeployment.Spec.Replicas + int32(surge) + c.safetyOptions.SafetyUp
				lowerThreshold = higherThreshold - c.safetyOptions.SafetyDown
			}
		}

		glog.V(3).Infof(
			"MS:%q LowerThreshold:%d FullyLabeledReplicas:%d HigherThreshold:%d",
			machineSet.Name,
			lowerThreshold,
			fullyLabeledReplicasCount,
			higherThreshold,
		)

		machineSetFrozenCondition := GetCondition(&machineSet.Status, v1alpha1.MachineSetFrozen)

		if machineSet.Labels["freeze"] != "True" &&
			fullyLabeledReplicasCount >= higherThreshold {
			message := fmt.Sprintf(
				"The number of machines backing MachineSet: %s is %d >= %d which is the Max-ScaleUp-Limit",
				machineSet.Name,
				fullyLabeledReplicasCount,
				higherThreshold,
			)
			return c.freezeMachineSetsAndDeployments(machineSet, OverShootingReplicaCount, message)

		} else if fullyLabeledReplicasCount <= lowerThreshold &&
			(machineSet.Labels["freeze"] == "True" || machineSetFrozenCondition != nil) {
			// Unfreeze if number of replicas is less than or equal to lowerThreshold
			// and freeze label or condition exists on machineSet
			return c.unfreezeMachineSetsAndDeployments(machineSet)
		}
	}
	return nil
}

// checkVMObjects checks for orphan VMs (VMs that don't have a machine object backing)
func (c *controller) checkVMObjects() {
	c.checkAWSMachineClass()
	c.checkOSMachineClass()
	c.checkAzureMachineClass()
	c.checkGCPMachineClass()
	c.checkAlicloudMachineClass()
	c.checkPacketMachineClass()
}

// checkAWSMachineClass checks for orphan VMs in AWSMachinesClasses
func (c *controller) checkAWSMachineClass() {
	AWSMachineClasses, err := c.awsMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range AWSMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkOSMachineClass checks for orphan VMs in OSMachinesClasses
func (c *controller) checkOSMachineClass() {
	OSMachineClasses, err := c.openStackMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range OSMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkOSMachineClass checks for orphan VMs in AzureMachinesClasses
func (c *controller) checkAzureMachineClass() {
	AzureMachineClasses, err := c.azureMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range AzureMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkGCPMachineClass checks for orphan VMs in GCPMachinesClasses
func (c *controller) checkGCPMachineClass() {
	GCPMachineClasses, err := c.gcpMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range GCPMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkAlicloudMachineClass checks for orphan VMs in AlicloudMachinesClasses
func (c *controller) checkAlicloudMachineClass() {
	AlicloudMachineClasses, err := c.alicloudMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range AlicloudMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkPacketMachineClass checks for orphan VMs in PacketMachinesClasses
func (c *controller) checkPacketMachineClass() {
	PacketMachineClasses, err := c.packetMachineClassLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range PacketMachineClasses {

		var machineClassInterface interface{}
		machineClassInterface = machineClass

		c.checkMachineClass(
			machineClassInterface,
			machineClass.Spec.SecretRef,
			machineClass.Name,
			machineClass.Kind,
		)
	}
}

// checkMachineClass checks a particular machineClass for orphan instances
func (c *controller) checkMachineClass(
	machineClass interface{},
	secretRef *corev1.SecretReference,
	className string,
	classKind string) {

	// Get secret
	secret, err := c.getSecret(secretRef, className)
	if err != nil || secret == nil {
		glog.Errorf("Secret reference not found for MachineClass: %q", className)
		return
	}

	// Dummy driver object being created to invoke GetVMs
	dvr := driver.NewDriver(
		"",
		secret,
		classKind,
		machineClass,
		"",
	)
	listOfVMs, err := dvr.GetVMs("")
	if err != nil {
		glog.Warningf("Failed to list VMs at provider. Err - %s", err)
	}

	// Making sure that its not a VM just being created, machine object not yet updated at API server
	if len(listOfVMs) > 1 {
		stopCh := make(chan struct{})
		if !cache.WaitForCacheSync(stopCh, c.machineSynced) {
			glog.Error("Timed out waiting for caches to sync - ", err)
			return
		}
	}

	for machineID, machineName := range listOfVMs {
		machine, err := c.machineLister.Machines(c.namespace).Get(machineName)

		if err != nil && !apierrors.IsNotFound(err) {
			// Any other types of errors
			glog.Error("Safety-Net: Error getting machines - ", err)
		} else if err != nil || machine.Spec.ProviderID != machineID {

			// If machine exists and machine object is still been processed by the machine controller
			if err == nil &&
				machine.Status.CurrentStatus.Phase == "" {
				glog.V(3).Infof("Machine object %q is being processed by machine controller, hence skipping", machine.Name)
				continue
			}

			// Re-check VM object existence
			// before deleting orphan VM
			result, _ := dvr.GetVMs(machineID)
			for reMachineID := range result {
				if reMachineID == machineID {
					// Get latest version of machine object and verfiy again
					machine, err := c.controlMachineClient.Machines(c.namespace).Get(machineName, metav1.GetOptions{})
					if (err != nil && apierrors.IsNotFound(err)) || machine.Spec.ProviderID != machineID {
						vm := make(map[string]string)
						vm[machineID] = machineName
						c.deleteOrphanVM(vm, secret, classKind, machineClass)
					}
				}
			}

		}
	}
}

// addMachineSetToSafety enqueues into machineSafetyQueue when a new machineSet is added
func (c *controller) addMachineSetToSafety(obj interface{}) {
	machineSet := obj.(*v1alpha1.MachineSet)
	c.updateTimeStamp(machineSet)
}

// updateMachineSetToSafety adds update timestamp
func (c *controller) updateMachineSetToSafety(old, new interface{}) {
	oldMS := old.(*v1alpha1.MachineSet)
	newMS := new.(*v1alpha1.MachineSet)
	if oldMS.Spec.Replicas != newMS.Spec.Replicas {
		c.updateTimeStamp(newMS)
	}
}

// addMachineToSafety enqueues into machineSafetyQueue when a new machine is added
func (c *controller) addMachineToSafety(obj interface{}) {
	machine := obj.(*v1alpha1.Machine)
	c.enqueueMachineSafetyOvershootingKey(machine)
}

// deleteMachineToSafety enqueues into machineSafetyQueue when a new machine is deleted
func (c *controller) deleteMachineToSafety(obj interface{}) {
	machine := obj.(*v1alpha1.Machine)
	c.enqueueMachineSafetyOrphanVMsKey(machine)
}

// updateTimeStamp adds an annotation indicating the last time the number of replicas
// of machineSet was changed
func (c *controller) updateTimeStamp(ms *v1alpha1.MachineSet) {
	for {
		// Get the latest version of the machineSet so that we can avoid conflicts
		ms, err := c.controlMachineClient.MachineSets(ms.Namespace).Get(ms.Name, metav1.GetOptions{})
		if err != nil {
			// Some error occurred while fetching object from API server
			glog.Error(err)
			break
		}
		clone := ms.DeepCopy()
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[LastReplicaUpdate] = metav1.Now().Format("2006-01-02 15:04:05 MST")
		_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
		if err == nil {
			break
		}
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying - ", err)
	}
}

// enqueueMachineSafetyOvershootingKey enqueues into machineSafetyOvershootingQueue
func (c *controller) enqueueMachineSafetyOvershootingKey(obj interface{}) {
	c.machineSafetyOvershootingQueue.Add("")
}

// enqueueMachineSafetyOrphanVMsKey enqueues into machineSafetyOrphanVMsQueue
func (c *controller) enqueueMachineSafetyOrphanVMsKey(obj interface{}) {
	c.machineSafetyOrphanVMsQueue.Add("")
}

// deleteOrphanVM teriminates's the VM on the cloud provider passed to it
func (c *controller) deleteOrphanVM(vm driver.VMs, secretRef *corev1.Secret, kind string, machineClass interface{}) {

	var machineID string
	var machineName string

	for k, v := range vm {
		machineID = k
		machineName = v
	}

	dvr := driver.NewDriver(
		machineID,
		secretRef,
		kind,
		machineClass,
		machineName,
	)

	err := dvr.Delete()
	if err != nil {
		glog.Errorf("Error while deleting VM on CP - %s. Shall retry in next safety controller sync.", err)
	} else {
		glog.V(2).Infof("Orphan VM found and terminated VM: %s, %s", machineName, machineID)
	}
}

// freezeMachineSetsAndDeployments freezes machineSets and machineDeployment (who is the owner of the machineSet)
func (c *controller) freezeMachineSetsAndDeployments(machineSet *v1alpha1.MachineSet, reason string, message string) error {

	glog.V(2).Infof("checkAndFreezeORUnfreezeMachineSets: Freezing MachineSet %q due to %q", machineSet.Name, reason)

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		glog.Warningf("checkAndFreezeORUnfreezeMachineSets: Failed to fetch machineSet. Error: %s", err)
		return err
	}

	clone := machineSet.DeepCopy()
	newStatus := clone.Status
	mscond := NewMachineSetCondition(v1alpha1.MachineSetFrozen, v1alpha1.ConditionTrue, reason, message)
	SetCondition(&newStatus, mscond)
	clone.Status = newStatus
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		glog.Warningf("checkAndFreezeORUnfreezeMachineSets: MachineSet/status update failed. Error: %s", err)
		return err
	}

	clone = machineSet.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	clone.Labels["freeze"] = "True"
	_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
	if err != nil {
		glog.Warningf("checkAndFreezeORUnfreezeMachineSets: MachineSet update failed. Error: %s", err)
		return err
	}

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		if machineDeployment != nil {

			// Get the latest version of the machineDeployment so that we can avoid conflicts
			machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
			if err != nil {
				glog.Warningf("checkAndFreezeORUnfreezeMachineSets: Failed to fetch machineDeployment. Error: %s", err)
				return err
			}

			clone := machineDeployment.DeepCopy()
			newStatus := clone.Status
			mdcond := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentFrozen, v1alpha1.ConditionTrue, reason, message)
			SetMachineDeploymentCondition(&newStatus, *mdcond)
			clone = machineDeployment.DeepCopy()
			clone.Status = newStatus
			machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(clone)
			if err != nil {
				glog.Warningf("checkAndFreezeORUnfreezeMachineSets: MachineDeployment/status update failed. Error: %s", err)
				return err
			}

			clone = machineDeployment.DeepCopy()
			if clone.Labels == nil {
				clone.Labels = make(map[string]string)
			}
			clone.Labels["freeze"] = "True"
			_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
			if err != nil {
				glog.Warningf("checkAndFreezeORUnfreezeMachineSets: MachineDeployment update failed. Error: %s", err)
				return err
			}

		}
	}
	return nil
}

// unfreezeMachineSetsAndDeployments unfreezes machineSets and machineDeployment (who is the owner of the machineSet)
func (c *controller) unfreezeMachineSetsAndDeployments(machineSet *v1alpha1.MachineSet) error {

	glog.V(2).Infof("UnFreezing MachineSet %q due to lesser than lower threshold replicas", machineSet.Name)

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		if machineDeployment != nil {

			// Get the latest version of the machineDeployment so that we can avoid conflicts
			machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
			if err != nil {
				// Some error occued while fetching object from API server
				glog.Warningf("checkAndFreezeORUnfreezeMachineSets: Failed to fetch machineDeployment. Error: %s", err)
				return err
			}

			clone := machineDeployment.DeepCopy()
			newStatus := clone.Status
			RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentFrozen)
			clone.Status = newStatus
			machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(clone)
			if err != nil {
				glog.Warningf("MachineDeployment/status update failed. Error: %s", err)
				return err
			}

			clone = machineDeployment.DeepCopy()
			if clone.Labels == nil {
				clone.Labels = make(map[string]string)
			}
			delete(clone.Labels, "freeze")
			_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
			if err != nil {
				glog.Warningf("MachineDeployment update failed. Error: %s", err)
				return err
			}

		}
	}

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		glog.Warningf("checkAndFreezeORUnfreezeMachineSets: Failed to fetch machineSet. Error: %s", err)
		return err
	}

	clone := machineSet.DeepCopy()
	newStatus := clone.Status
	RemoveCondition(&newStatus, v1alpha1.MachineSetFrozen)
	clone.Status = newStatus
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		glog.Warningf("MachineSet/status update failed. Error: %s", err)
		return err
	}

	clone = machineSet.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	delete(clone.Labels, "freeze")
	_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
	if err != nil {
		glog.Warningf("MachineSet update failed. Error: %s", err)
		return err
	}

	return nil
}
