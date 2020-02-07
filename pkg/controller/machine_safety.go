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

	klog.V(4).Infof("reconcileClusterMachineSafetyOrphanVMs: Start")
	defer klog.V(4).Infof("reconcileClusterMachineSafetyOrphanVMs: End, reSync-Period: %v", reSyncAfter)

	c.checkVMObjects()

	return nil
}

// reconcileClusterMachineSafetyOvershooting checks all machineSet/machineDeployment
// if the number of machine objects backing them is way beyond its desired replicas
func (c *controller) reconcileClusterMachineSafetyOvershooting(key string) error {
	stopCh := make(chan struct{})
	defer close(stopCh)

	reSyncAfter := c.safetyOptions.MachineSafetyOvershootingPeriod.Duration
	defer c.machineSafetyOvershootingQueue.AddAfter("", reSyncAfter)

	klog.V(4).Infof("reconcileClusterMachineSafetyOvershooting: Start")
	defer klog.V(4).Infof("reconcileClusterMachineSafetyOvershooting: End, reSync-Period: %v", reSyncAfter)

	err := c.checkAndFreezeORUnfreezeMachineSets()
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineSetSynced, c.machineDeploymentSynced)

	err = c.syncMachineDeploymentFreezeState()
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineDeploymentSynced)

	err = c.unfreezeMachineDeploymentsWithUnfreezeAnnotation()
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineSetSynced)

	err = c.unfreezeMachineSetsWithUnfreezeAnnotation()
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}

	return err
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

// unfreezeMachineDeploymentsWithUnfreezeAnnotation unfreezes machineDeployment with unfreeze annotation
func (c *controller) unfreezeMachineDeploymentsWithUnfreezeAnnotation() error {
	machineDeployments, err := c.machineDeploymentLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineDeployments - ", err)
		return err
	}

	for _, machineDeployment := range machineDeployments {
		if _, exists := machineDeployment.Annotations[UnfreezeAnnotation]; exists {
			klog.V(2).Infof("SafetyController: UnFreezing MachineDeployment %q due to setting unfreeze annotation", machineDeployment.Name)

			err := c.unfreezeMachineDeployment(machineDeployment, "UnfreezeAnnotation")
			if err != nil {
				return err
			}

			// Apply UnfreezeAnnotation on all machineSets backed by the machineDeployment
			machineSets, err := c.getMachineSetsForMachineDeployment(machineDeployment)
			if err == nil {
				for _, machineSet := range machineSets {
					// Get the latest version of the machineSet so that we can avoid conflicts
					machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
					if err != nil {
						// Some error occued while fetching object from API server
						klog.Errorf("SafetyController: Failed to GET machineSet. Error: %s", err)
						return err
					}
					clone := machineSet.DeepCopy()
					if clone.Annotations == nil {
						clone.Annotations = make(map[string]string, 0)
					}
					clone.Annotations[UnfreezeAnnotation] = "True"
					machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
					if err != nil {
						klog.Errorf("SafetyController: MachineSet UPDATE failed. Error: %s", err)
						return err
					}
				}
			}
		}
	}

	return nil
}

// unfreezeMachineSetsWithUnfreezeAnnotation unfreezes machineSets with unfreeze annotation
func (c *controller) unfreezeMachineSetsWithUnfreezeAnnotation() error {
	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineSets - ", err)
		return err
	}

	for _, machineSet := range machineSets {
		if _, exists := machineSet.Annotations[UnfreezeAnnotation]; exists {
			klog.V(2).Infof("SafetyController: UnFreezing MachineSet %q due to setting unfreeze annotation", machineSet.Name)

			err := c.unfreezeMachineSet(machineSet)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// syncMachineDeploymentFreezeState syncs freeze labels and conditions to keep it consistent
func (c *controller) syncMachineDeploymentFreezeState() error {
	machineDeployments, err := c.machineDeploymentLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineDeployments - ", err)
		return err
	}

	for _, machineDeployment := range machineDeployments {

		machineDeploymentFreezeLabelPresent := (machineDeployment.Labels["freeze"] == "True")
		machineDeploymentFrozenConditionPresent := (GetMachineDeploymentCondition(machineDeployment.Status, v1alpha1.MachineDeploymentFrozen) != nil)

		machineDeploymentHasFrozenMachineSet := false
		machineSets, err := c.getMachineSetsForMachineDeployment(machineDeployment)
		if err == nil {
			for _, machineSet := range machineSets {
				machineSetFreezeLabelPresent := (machineSet.Labels["freeze"] == "True")
				machineSetFrozenConditionPresent := (GetCondition(&machineSet.Status, v1alpha1.MachineSetFrozen) != nil)

				if machineSetFreezeLabelPresent || machineSetFrozenConditionPresent {
					machineDeploymentHasFrozenMachineSet = true
					break
				}
			}
		}

		if machineDeploymentHasFrozenMachineSet {
			// If machineDeployment has atleast one frozen machine set backing it

			if !machineDeploymentFreezeLabelPresent || !machineDeploymentFrozenConditionPresent {
				// Either the freeze label or freeze condition is not present on the machineDeployment
				message := "MachineDeployment State was inconsistent, hence safety controller has fixed this and frozen it"
				err := c.freezeMachineDeployment(machineDeployment, MachineDeploymentStateSync, message)
				if err != nil {
					return err
				}
			}
		} else {
			// If machineDeployment has no frozen machine set backing it

			if machineDeploymentFreezeLabelPresent || machineDeploymentFrozenConditionPresent {
				// Either the freeze label or freeze condition is present present on the machineDeployment
				err := c.unfreezeMachineDeployment(machineDeployment, MachineDeploymentStateSync)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// checkAndFreezeORUnfreezeMachineSets freezes/unfreezes machineSets/machineDeployments
// which have much greater than desired number of replicas of machine objects
func (c *controller) checkAndFreezeORUnfreezeMachineSets() error {
	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineSets - ", err)
		return err
	}

	for _, machineSet := range machineSets {

		filteredMachines, err := c.machineLister.List(labels.Everything())
		if err != nil {
			klog.Error("SafetyController: Error while trying to LIST machines - ", err)
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
					klog.Error("SafetyController: Error while trying to GET surge value - ", err)
					return err
				}

				higherThreshold = machineDeployment.Spec.Replicas + int32(surge) + c.safetyOptions.SafetyUp
				lowerThreshold = higherThreshold - c.safetyOptions.SafetyDown
			}
		}

		klog.V(4).Infof(
			"checkAndFreezeORUnfreezeMachineSets: MS:%q LowerThreshold:%d FullyLabeledReplicas:%d HigherThreshold:%d",
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
			return c.freezeMachineSetAndDeployment(machineSet, OverShootingReplicaCount, message)

		} else if fullyLabeledReplicasCount <= lowerThreshold &&
			(machineSet.Labels["freeze"] == "True" || machineSetFrozenCondition != nil) {
			// Unfreeze if number of replicas is less than or equal to lowerThreshold
			// and freeze label or condition exists on machineSet
			return c.unfreezeMachineSetAndDeployment(machineSet)
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
	c.checkMetalMachineClass()
}

// checkAWSMachineClass checks for orphan VMs in AWSMachinesClasses
func (c *controller) checkAWSMachineClass() {
	AWSMachineClasses, err := c.awsMachineClassLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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
		klog.Error("SafetyController: Error while trying to LIST machineClasses ", err)
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

// checkMetalMachineClass checks for orphan Machines in MetalMachinesClasses
func (c *controller) checkMetalMachineClass() {
	MetalMachineClasses, err := c.metalMachineClassLister.List(labels.Everything())
	if err != nil {
		klog.Error("Safety-Net: Error getting machineClasses")
		return
	}

	for _, machineClass := range MetalMachineClasses {

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
		klog.Errorf("SafetyController: Secret reference not found for MachineClass: %q", className)
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
		klog.Errorf("SafetyController: Failed to LIST VMs at provider. Error: %s", err)
	}

	// Making sure that its not a VM just being created, machine object not yet updated at API server
	if len(listOfVMs) > 1 {
		stopCh := make(chan struct{})
		defer close(stopCh)

		if !cache.WaitForCacheSync(stopCh, c.machineSynced) {
			klog.Errorf("SafetyController: Timed out waiting for caches to sync. Error: %s", err)
			return
		}
	}

	for machineID, machineName := range listOfVMs {
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

	err := dvr.Delete(machineID)
	if err != nil {
		klog.Errorf("SafetyController: Error while trying to DELETE VM on CP - %s. Shall retry in next safety controller sync.", err)
	} else {
		klog.V(2).Infof("SafetyController: Orphan VM found and terminated VM: %s, %s", machineName, machineID)
	}
}

// freezeMachineSetAndDeployment freezes machineSet and machineDeployment (who is the owner of the machineSet)
func (c *controller) freezeMachineSetAndDeployment(machineSet *v1alpha1.MachineSet, reason string, message string) error {

	klog.V(2).Infof("SafetyController: Freezing MachineSet %q due to %q", machineSet.Name, reason)

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		klog.Errorf("SafetyController: Failed to GET machineSet. Error: %s", err)
		return err
	}

	clone := machineSet.DeepCopy()
	newStatus := clone.Status
	mscond := NewMachineSetCondition(v1alpha1.MachineSetFrozen, v1alpha1.ConditionTrue, reason, message)
	SetCondition(&newStatus, mscond)
	clone.Status = newStatus
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineSet/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineSet.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	clone.Labels["freeze"] = "True"
	_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineSet UPDATE failed. Error: %s", err)
		return err
	}

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		if machineDeployment != nil {
			err := c.freezeMachineDeployment(machineDeployment, reason, message)
			if err != nil {
				return err
			}
		}
	}

	klog.V(2).Infof("SafetyController: Froze MachineSet %q due to overshooting of replicas", machineSet.Name)
	return nil
}

// unfreezeMachineSetAndDeployment unfreezes machineSets and machineDeployment (who is the owner of the machineSet)
func (c *controller) unfreezeMachineSetAndDeployment(machineSet *v1alpha1.MachineSet) error {

	klog.V(2).Infof("SafetyController: UnFreezing MachineSet %q due to lesser than lower threshold replicas", machineSet.Name)

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		err := c.unfreezeMachineDeployment(machineDeployment, "UnderShootingReplicaCount")
		if err != nil {
			return err
		}
	}

	err := c.unfreezeMachineSet(machineSet)
	if err != nil {
		return err
	}

	return nil
}

// unfreezeMachineSetsAndDeployments unfreezes machineSets
func (c *controller) unfreezeMachineSet(machineSet *v1alpha1.MachineSet) error {

	if machineSet == nil {
		err := fmt.Errorf("SafetyController: Machine Set not passed")
		klog.Errorf(err.Error())
		return err
	}

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		klog.Errorf("SafetyController: Failed to GET machineSet. Error: %s", err)
		return err
	}

	clone := machineSet.DeepCopy()
	newStatus := clone.Status
	RemoveCondition(&newStatus, v1alpha1.MachineSetFrozen)
	clone.Status = newStatus
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineSet/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineSet.DeepCopy()
	if clone.Annotations == nil {
		clone.Annotations = make(map[string]string)
	}
	delete(clone.Annotations, UnfreezeAnnotation)
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	delete(clone.Labels, "freeze")
	_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineSet UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Unfroze MachineSet %q", machineSet.Name)
	return nil
}

// freezeMachineDeployment freezes the machineDeployment
func (c *controller) freezeMachineDeployment(machineDeployment *v1alpha1.MachineDeployment, reason string, message string) error {
	// Get the latest version of the machineDeployment so that we can avoid conflicts
	machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("SafetyController: Failed to GET machineDeployment. Error: %s", err)
		return err
	}

	clone := machineDeployment.DeepCopy()
	newStatus := clone.Status
	mdcond := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentFrozen, v1alpha1.ConditionTrue, reason, message)
	SetMachineDeploymentCondition(&newStatus, *mdcond)
	clone.Status = newStatus
	machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineDeployment.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	clone.Labels["freeze"] = "True"
	_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Froze MachineDeployment %q due to %s", machineDeployment.Name, reason)
	return nil
}

// unfreezeMachineDeployment unfreezes the machineDeployment
func (c *controller) unfreezeMachineDeployment(machineDeployment *v1alpha1.MachineDeployment, reason string) error {

	if machineDeployment == nil {
		err := fmt.Errorf("SafetyController: Machine Deployment not passed")
		klog.Errorf(err.Error())
		return err
	}

	// Get the latest version of the machineDeployment so that we can avoid conflicts
	machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		klog.Errorf("SafetyController: Failed to GET machineDeployment. Error: %s", err)
		return err
	}

	clone := machineDeployment.DeepCopy()
	newStatus := clone.Status
	RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentFrozen)
	clone.Status = newStatus
	machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineDeployment.DeepCopy()
	if clone.Annotations == nil {
		clone.Annotations = make(map[string]string)
	}
	delete(clone.Annotations, UnfreezeAnnotation)
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	delete(clone.Labels, "freeze")
	_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Unfroze MachineDeployment %q due to %s", machineDeployment.Name, reason)
	return nil
}
