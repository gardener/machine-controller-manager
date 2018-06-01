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
	"sync"
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

// SafetyCheck controller is used to protect the controller from orphan or excess VMs being created
func (c *controller) reconcileClusterMachineSafety(key string) error {
	var wg sync.WaitGroup

	glog.V(3).Info("SafetyCheck loop initializing")
	wg.Add(2)
	go c.checkAndFreezeORUnfreezeMachineSets(&wg)
	go c.checkVMObjects(&wg)
	//Disable permenant freeze for now. We should enable it again once we have sophisticated automatic unfreeze mechanism in place.
	//go c.checkAndFreezeMachineSetTimeout(&wg)
	wg.Wait()
	c.machineSafetyQueue.AddAfter("", 60*time.Second)

	return nil
}

// checkAndFreezeORUnfreezeMachineSets freezes/unfreezes machineSets/machineDeployments
// which have much greater than desired number of replicas of machine objects
func (c *controller) checkAndFreezeORUnfreezeMachineSets(wg *sync.WaitGroup) {

	defer wg.Done()

	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineSets - ", err)
		return
	}

	for _, machineSet := range machineSets {

		filteredMachines, err := c.machineLister.List(labels.Everything())
		if err != nil {
			glog.Error("Safety-Net: Error getting machines - ", err)
			return
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
					glog.Error("Safety-Net: Error getting surge value - ", err)
					return
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

		if machineSet.Labels["freeze"] != "True" &&
			fullyLabeledReplicasCount >= higherThreshold {
			message := fmt.Sprintf(
				"The number of machines backing MachineSet: %s is %d >= %d which is the Max-ScaleUp-Limit",
				machineSet.Name,
				fullyLabeledReplicasCount,
				higherThreshold,
			)
			c.freezeMachineSetsAndDeployments(machineSet, OverShootingReplicaCount, message)

		} else if machineSet.Labels["freeze"] == "True" &&
			//TODO: Reintroduce this checks once we have automated unfreeze for MachinTimeout aka meltdown.
			//machineSet.Status.Conditions != nil &&
			//GetCondition(&machineSet.Status, v1alpha1.MachineSetFrozen).Reason == OverShootingReplicaCount &&
			fullyLabeledReplicasCount <= lowerThreshold {
			c.unfreezeMachineSetsAndDeployments(machineSet)
		}
	}
}

// checkVMObjects checks for orphan VMs (VMs that don't have a machine object backing)
func (c *controller) checkVMObjects(wg *sync.WaitGroup) {
	go c.checkAWSMachineClass()
	go c.checkOSMachineClass()
	go c.checkAzureMachineClass()
	go c.checkGCPMachineClass()

	wg.Done()
}

// checkAndFreezeMachineSetTimeout permanently freezes any
// machineSet/machineDeployment whose creation times out
func (c *controller) checkAndFreezeMachineSetTimeout(wg *sync.WaitGroup) {

	timeout := time.Duration(c.safetyOptions.MachineSetScaleTimeout) * time.Minute

	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		glog.Error("Safety-Net: Error getting machineSets - ", err)
		wg.Done()
		return
	}

	for _, ms := range machineSets {
		if ms.Annotations != nil {
			timestampString, ok := ms.Annotations[LastReplicaUpdate]
			if ok {

				if ms.Labels["freeze"] == "True" &&
					ms.Status.Conditions != nil &&
					GetCondition(&ms.Status, v1alpha1.MachineSetFrozen).Reason == TimeoutOccurred {
					// MachineSet already frozen permanently due to timeout
					continue
				}

				layout := "2006-01-02 15:04:05 MST"
				timestamp, err := time.Parse(layout, timestampString)
				if err != nil {
					glog.Error("Error parsing time: ", err)
					wg.Done()
					return
				}

				if ms.Status.ReadyReplicas == ms.Spec.Replicas {
					for {
						// Get the latest version of the machineSet so that we can avoid conflicts
						ms, err := c.controlMachineClient.MachineSets(ms.Namespace).Get(ms.Name, metav1.GetOptions{})
						if err != nil {
							// Some error occued while fetching object from API server
							glog.Error(err)
							break
						}
						clone := ms.DeepCopy()
						delete(clone.Annotations, LastReplicaUpdate)
						_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
						if err == nil {
							break
						}
						// Keep retrying until update goes through
						glog.Warning("Updated failed, retrying - ", err)
					}
				} else if time.Since(timestamp) > timeout {
					message := "MachineSet has timed out while scaling replicas"
					c.freezeMachineSetsAndDeployments(ms, TimeoutOccurred, message)
				}
			}
		}
	}

	wg.Done()
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

			// Re-check VM object existance
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
	c.enqueueMachineSafetyKey(machine)
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

// enqueueMachineSafetyKey enqueues into machineSafetyQueue
func (c *controller) enqueueMachineSafetyKey(obj interface{}) {
	c.machineSafetyQueue.Add("")
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
func (c *controller) freezeMachineSetsAndDeployments(machineSet *v1alpha1.MachineSet, reason string, message string) {

	glog.V(2).Infof("Freezing MachineSet %q due to %q", machineSet.Name, reason)

	for {
		// TODO: Replace it with better retry logic. Replace all occurrences similarly.
		// Ref: https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/deployment/util/replicaset_util.go#L35
		// Get the latest version of the machineSet so that we can avoid conflicts
		machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
		if err != nil {
			// Some error occued while fetching object from API server
			glog.Error(err)
			break
		}
		clone := machineSet.DeepCopy()
		newStatus := clone.Status
		mscond := NewMachineSetCondition(v1alpha1.MachineSetFrozen, v1alpha1.ConditionTrue, reason, message)
		SetCondition(&newStatus, mscond)
		if clone.Labels == nil {
			clone.Labels = make(map[string]string)
		}
		clone.Status = newStatus
		clone.Labels["freeze"] = "True"
		_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
		if err == nil {
			break
		}
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying - ", err)
	}

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		if machineDeployment != nil {
			for {
				// Get the latest version of the machineDeployment so that we can avoid conflicts
				machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
				if err != nil {
					// Some error occued while fetching object from API server
					//TODO explore if we can log/annotate this machinedeployment and continue here.
					glog.Error(err)
					break
				}
				clone := machineDeployment.DeepCopy()
				newStatus := clone.Status
				mdcond := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentFrozen, v1alpha1.ConditionTrue, reason, message)
				SetMachineDeploymentCondition(&newStatus, *mdcond)
				if clone.Labels == nil {
					clone.Labels = make(map[string]string)
				}
				clone.Status = newStatus
				clone.Labels["freeze"] = "True"
				_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
				if err == nil {
					break
				}
				// Keep retrying until update goes through
				glog.Warning("Updated failed, retrying - ", err)
			}
		}
	}
}

// unfreezeMachineSetsAndDeployments unfreezes machineSets and machineDeployment (who is the owner of the machineSet)
func (c *controller) unfreezeMachineSetsAndDeployments(machineSet *v1alpha1.MachineSet) {

	glog.V(2).Infof("UnFreezing MachineSet %q due to lesser than lower threshold replicas", machineSet.Name)

	for {
		// Get the latest version of the machineSet so that we can avoid conflicts
		machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(machineSet.Name, metav1.GetOptions{})
		if err != nil {
			// Some error occued while fetching object from API server
			glog.Error(err)
			break
		}
		clone := machineSet.DeepCopy()
		newStatus := clone.Status
		RemoveCondition(&newStatus, v1alpha1.MachineSetFrozen)
		clone.Status = newStatus
		delete(clone.Labels, "freeze")
		_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(clone)
		if err == nil {
			break
		}
		// Keep retrying until update goes through
		glog.Warning("Updated failed, retrying - ", err)
	}

	machineDeployment := c.getMachineDeploymentsForMachineSet(machineSet)[0]
	if machineDeployment != nil {
		for {
			// Get the latest version of the machineDeployment so that we can avoid conflicts
			machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(machineDeployment.Name, metav1.GetOptions{})
			if err != nil {
				// Some error occued while fetching object from API server
				glog.Error(err)
				break
			}
			clone := machineDeployment.DeepCopy()
			if clone.Labels == nil {
				clone.Labels = make(map[string]string)
			}
			newStatus := clone.Status
			RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentFrozen)
			clone.Status = newStatus
			delete(clone.Labels, "freeze")
			_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(clone)
			if err == nil {
				break
			}
			// Keep retrying until update goes through
			glog.Warning("Updated failed, retrying - ", err)
		}
	}
}
