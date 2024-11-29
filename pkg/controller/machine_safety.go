// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/cache"

	"k8s.io/klog/v2"
)

const (
	// OverShootingReplicaCount freeze reason when replica count overshoots
	OverShootingReplicaCount = "OverShootingReplicaCount"
	// MachineDeploymentStateSync freeze reason when machineDeployment was found with inconsistent state
	MachineDeploymentStateSync = "MachineDeploymentStateSync"
	// UnfreezeAnnotation indicates the controllers to unfreeze this object
	UnfreezeAnnotation = "safety.machine.sapcloud.io/unfreeze"
)

// reconcileClusterMachineSafetyOvershooting checks all machineSet/machineDeployment
// if the number of machine objects backing them is way beyond its desired replicas
func (c *controller) reconcileClusterMachineSafetyOvershooting(_ string) error {
	ctx := context.Background()
	stopCh := make(chan struct{})
	defer close(stopCh)

	reSyncAfter := c.safetyOptions.MachineSafetyOvershootingPeriod.Duration
	defer c.machineSafetyOvershootingQueue.AddAfter("", reSyncAfter)

	klog.V(4).Infof("reconcileClusterMachineSafetyOvershooting: Start")
	defer klog.V(4).Infof("reconcileClusterMachineSafetyOvershooting: End, reSync-Period: %v", reSyncAfter)

	err := c.checkAndFreezeORUnfreezeMachineSets(ctx)
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineSetSynced, c.machineDeploymentSynced)

	err = c.syncMachineDeploymentFreezeState(ctx)
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineDeploymentSynced)

	err = c.unfreezeMachineDeploymentsWithUnfreezeAnnotation(ctx)
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}
	cache.WaitForCacheSync(stopCh, c.machineSetSynced)

	err = c.unfreezeMachineSetsWithUnfreezeAnnotation(ctx)
	if err != nil {
		klog.Errorf("SafetyController: %v", err)
	}

	return err
}

// unfreezeMachineDeploymentsWithUnfreezeAnnotation unfreezes machineDeployment with unfreeze annotation
func (c *controller) unfreezeMachineDeploymentsWithUnfreezeAnnotation(ctx context.Context) error {
	machineDeployments, err := c.machineDeploymentLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineDeployments - ", err)
		return err
	}

	for _, machineDeployment := range machineDeployments {
		if _, exists := machineDeployment.Annotations[UnfreezeAnnotation]; exists {
			klog.V(2).Infof("SafetyController: UnFreezing MachineDeployment %q due to setting unfreeze annotation", machineDeployment.Name)

			err := c.unfreezeMachineDeployment(ctx, machineDeployment, "UnfreezeAnnotation")
			if err != nil {
				return err
			}

			// Apply UnfreezeAnnotation on all machineSets backed by the machineDeployment
			machineSets, err := c.getMachineSetsForMachineDeployment(ctx, machineDeployment)
			if err == nil {
				for _, machineSet := range machineSets {
					// Get the latest version of the machineSet so that we can avoid conflicts
					machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(ctx, machineSet.Name, metav1.GetOptions{})
					if err != nil {
						// Some error occued while fetching object from API server
						klog.Errorf("SafetyController: Failed to GET machineSet. Error: %s", err)
						return err
					}
					clone := machineSet.DeepCopy()
					if clone.Annotations == nil {
						clone.Annotations = make(map[string]string)
					}
					clone.Annotations[UnfreezeAnnotation] = "True"
					machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
					if err != nil {
						klog.Errorf("SafetyController: MachineSet %s UPDATE failed. Error: %s", machineSet.Name, err)
						return err
					}
				}
			}
		}
	}

	return nil
}

// unfreezeMachineSetsWithUnfreezeAnnotation unfreezes machineSets with unfreeze annotation
func (c *controller) unfreezeMachineSetsWithUnfreezeAnnotation(ctx context.Context) error {
	machineSets, err := c.machineSetLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineSets - ", err)
		return err
	}

	for _, machineSet := range machineSets {
		if _, exists := machineSet.Annotations[UnfreezeAnnotation]; exists {
			klog.V(2).Infof("SafetyController: UnFreezing MachineSet %q due to setting unfreeze annotation", machineSet.Name)

			err := c.unfreezeMachineSet(ctx, machineSet)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// syncMachineDeploymentFreezeState syncs freeze labels and conditions to keep it consistent
func (c *controller) syncMachineDeploymentFreezeState(ctx context.Context) error {
	machineDeployments, err := c.machineDeploymentLister.List(labels.Everything())
	if err != nil {
		klog.Error("SafetyController: Error while trying to LIST machineDeployments - ", err)
		return err
	}

	for _, machineDeployment := range machineDeployments {

		machineDeploymentFreezeLabelPresent := (machineDeployment.Labels["freeze"] == "True")
		machineDeploymentFrozenConditionPresent := (GetMachineDeploymentCondition(machineDeployment.Status, v1alpha1.MachineDeploymentFrozen) != nil)

		machineDeploymentHasFrozenMachineSet := false
		machineSets, err := c.getMachineSetsForMachineDeployment(ctx, machineDeployment)
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
				err := c.freezeMachineDeployment(ctx, machineDeployment, MachineDeploymentStateSync, message)
				if err != nil {
					return err
				}
			}
		} else {
			// If machineDeployment has no frozen machine set backing it

			if machineDeploymentFreezeLabelPresent || machineDeploymentFrozenConditionPresent {
				// Either the freeze label or freeze condition is present present on the machineDeployment
				err := c.unfreezeMachineDeployment(ctx, machineDeployment, MachineDeploymentStateSync)
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
func (c *controller) checkAndFreezeORUnfreezeMachineSets(ctx context.Context) error {
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
		// if we have a parent machineDeployment than we use a different higherThreshold and lowerThreshold,
		// keeping in mind the rolling update scenario, as we won't want to freeze during a normal rolling update.
		if len(machineDeployments) >= 1 {
			machineDeployment := machineDeployments[0]
			if machineDeployment != nil {
				var maxSurge *intstrutil.IntOrString
				if machineDeployment.Spec.Strategy.RollingUpdate != nil {
					maxSurge = machineDeployment.Spec.Strategy.RollingUpdate.MaxSurge
				} else if machineDeployment.Spec.Strategy.InPlaceUpdate != nil {
					maxSurge = machineDeployment.Spec.Strategy.InPlaceUpdate.MaxSurge
				}
				surge, err := intstrutil.GetValueFromIntOrPercent(
					maxSurge,
					int(machineDeployment.Spec.Replicas),
					true,
				)
				if err != nil {
					klog.Error("SafetyController: Error while trying to GET surge value - ", err)
					return err
				}
				higherThreshold = machineDeployment.Spec.Replicas + int32(surge) + c.safetyOptions.SafetyUp // #nosec G115 (CWE-190) -- value already validated
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
			return c.freezeMachineSetAndDeployment(ctx, machineSet, OverShootingReplicaCount, message)

		} else if fullyLabeledReplicasCount <= lowerThreshold &&
			(machineSet.Labels["freeze"] == "True" || machineSetFrozenCondition != nil) {
			// Unfreeze if number of replicas is less than or equal to lowerThreshold
			// and freeze label or condition exists on machineSet
			return c.unfreezeMachineSetAndDeployment(ctx, machineSet)
		}
	}
	return nil
}

// addMachineToSafetyOvershooting enqueues into machineSafetyOvershootingQueue when a new machine is added
func (c *controller) addMachineToSafetyOvershooting(obj interface{}) {
	machine := obj.(*v1alpha1.Machine)
	c.enqueueMachineSafetyOvershootingKey(machine)
}

// enqueueMachineSafetyOvershootingKey enqueues into machineSafetyOvershootingQueue
func (c *controller) enqueueMachineSafetyOvershootingKey(_ interface{}) {
	c.machineSafetyOvershootingQueue.Add("")
}

// freezeMachineSetAndDeployment freezes machineSet and machineDeployment (who is the owner of the machineSet)
func (c *controller) freezeMachineSetAndDeployment(ctx context.Context, machineSet *v1alpha1.MachineSet, reason string, message string) error {

	klog.V(2).Infof("SafetyController: Freezing MachineSet %q due to %q", machineSet.Name, reason)

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(ctx, machineSet.Name, metav1.GetOptions{})
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
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineSet/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineSet.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	clone.Labels["freeze"] = "True"
	_, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineSet UPDATE failed. Error: %s", err)
		return err
	}

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		if machineDeployment != nil {
			err := c.freezeMachineDeployment(ctx, machineDeployment, reason, message)
			if err != nil {
				return err
			}
		}
	}

	klog.V(2).Infof("SafetyController: Froze MachineSet %q due to overshooting of replicas", machineSet.Name)
	return nil
}

// unfreezeMachineSetAndDeployment unfreezes machineSets and machineDeployment (who is the owner of the machineSet)
func (c *controller) unfreezeMachineSetAndDeployment(ctx context.Context, machineSet *v1alpha1.MachineSet) error {

	klog.V(2).Infof("SafetyController: UnFreezing MachineSet %q due to lesser than lower threshold replicas", machineSet.Name)

	machineDeployments := c.getMachineDeploymentsForMachineSet(machineSet)
	if len(machineDeployments) >= 1 {
		machineDeployment := machineDeployments[0]
		err := c.unfreezeMachineDeployment(ctx, machineDeployment, "UnderShootingReplicaCount")
		if err != nil {
			return err
		}
	}

	err := c.unfreezeMachineSet(ctx, machineSet)
	if err != nil {
		return err
	}

	return nil
}

// unfreezeMachineSetsAndDeployments unfreezes machineSets
func (c *controller) unfreezeMachineSet(ctx context.Context, machineSet *v1alpha1.MachineSet) error {

	if machineSet == nil {
		err := fmt.Errorf("SafetyController: Machine Set not passed")
		klog.Errorf("%s", err.Error())
		return err
	}

	// Get the latest version of the machineSet so that we can avoid conflicts
	machineSet, err := c.controlMachineClient.MachineSets(machineSet.Namespace).Get(ctx, machineSet.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		klog.Errorf("SafetyController: Failed to GET machineSet. Error: %s", err)
		return err
	}

	clone := machineSet.DeepCopy()
	newStatus := clone.Status
	RemoveCondition(&newStatus, v1alpha1.MachineSetFrozen)
	clone.Status = newStatus
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
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
	machineSet, err = c.controlMachineClient.MachineSets(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineSet UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Unfroze MachineSet %q", machineSet.Name)
	return nil
}

// freezeMachineDeployment freezes the machineDeployment
func (c *controller) freezeMachineDeployment(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, reason string, message string) error {
	// Get the latest version of the machineDeployment so that we can avoid conflicts
	machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(ctx, machineDeployment.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("SafetyController: Failed to GET machineDeployment. Error: %s", err)
		return err
	}

	clone := machineDeployment.DeepCopy()
	newStatus := clone.Status
	mdcond := NewMachineDeploymentCondition(v1alpha1.MachineDeploymentFrozen, v1alpha1.ConditionTrue, reason, message)
	SetMachineDeploymentCondition(&newStatus, *mdcond)
	clone.Status = newStatus
	machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment/status UPDATE failed. Error: %s", err)
		return err
	}

	clone = machineDeployment.DeepCopy()
	if clone.Labels == nil {
		clone.Labels = make(map[string]string)
	}
	clone.Labels["freeze"] = "True"
	_, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Froze MachineDeployment %q due to %s", machineDeployment.Name, reason)
	return nil
}

// unfreezeMachineDeployment unfreezes the machineDeployment
func (c *controller) unfreezeMachineDeployment(ctx context.Context, machineDeployment *v1alpha1.MachineDeployment, reason string) error {

	if machineDeployment == nil {
		err := fmt.Errorf("SafetyController: Machine Deployment not passed")
		klog.Errorf("%s", err.Error())
		return err
	}

	// Get the latest version of the machineDeployment so that we can avoid conflicts
	machineDeployment, err := c.controlMachineClient.MachineDeployments(machineDeployment.Namespace).Get(ctx, machineDeployment.Name, metav1.GetOptions{})
	if err != nil {
		// Some error occued while fetching object from API server
		klog.Errorf("SafetyController: Failed to GET machineDeployment. Error: %s", err)
		return err
	}

	clone := machineDeployment.DeepCopy()
	newStatus := clone.Status
	RemoveMachineDeploymentCondition(&newStatus, v1alpha1.MachineDeploymentFrozen)
	clone.Status = newStatus
	machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).UpdateStatus(ctx, clone, metav1.UpdateOptions{})
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
	machineDeployment, err = c.controlMachineClient.MachineDeployments(clone.Namespace).Update(ctx, clone, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("SafetyController: MachineDeployment UPDATE failed. Error: %s", err)
		return err
	}

	klog.V(2).Infof("SafetyController: Unfroze MachineDeployment %q due to %s", machineDeployment.Name, reason)
	return nil
}
