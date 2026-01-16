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
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/controller/deployment/util/replicaset_util.go

Modifications Copyright SAP SE or an SAP affiliate company and Gardener contributors
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
	labelsutil "github.com/gardener/machine-controller-manager/pkg/util/labels"
	"github.com/gardener/machine-controller-manager/pkg/util/provider/machineutils"
)

// TODO: use client library instead when it starts to support update retries
//
//	see https://github.com/kubernetes/kubernetes/issues/21479
type updateISFunc func(is *v1alpha1.MachineSet) error

// UpdateISWithRetries updates a RS with given applyUpdate function. Note that RS not found error is ignored.
// The returned bool value can be used to tell if the RS is actually updated.
func UpdateISWithRetries(ctx context.Context, isClient v1alpha1client.MachineSetInterface, isLister v1alpha1listers.MachineSetLister, namespace, name string, applyUpdate updateISFunc) (*v1alpha1.MachineSet, error) {
	var is *v1alpha1.MachineSet

	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error
		is, err = isLister.MachineSets(namespace).Get(name)
		if err != nil {
			return err
		}
		is = is.DeepCopy()
		// Apply the update, then attempt to push it to the apiserver.
		if applyErr := applyUpdate(is); applyErr != nil {
			return applyErr
		}
		is, err = isClient.Update(ctx, is, metav1.UpdateOptions{})
		return err
	})

	// Ignore the precondition violated error, but the RS isn't updated.
	if retryErr == errorsutil.ErrPreconditionViolated {
		klog.V(4).Infof("Machine set %s precondition doesn't hold, skip updating it.", name)
		retryErr = nil
	}

	return is, retryErr
}

// GetMachineSetHash returns the hash of a machineSet
func GetMachineSetHash(is *v1alpha1.MachineSet, uniquifier *int32) (string, error) {
	isTemplate := is.Spec.Template.DeepCopy()
	isTemplate.Labels = labelsutil.CloneAndRemoveLabel(isTemplate.Labels, v1alpha1.DefaultMachineDeploymentUniqueLabelKey)
	return fmt.Sprintf("%d", ComputeHash(isTemplate, uniquifier)), nil
}

// syncMachinesNodeTemplates updates all machines in the given machineList with the new nodeTemplate if required.
func (c *controller) syncMachinesNodeTemplates(ctx context.Context, machineList []*v1alpha1.Machine, machineSet *v1alpha1.MachineSet) error {

	controlClient := c.controlMachineClient
	machineLister := c.machineLister

	for _, machine := range machineList {
		// Ignore inactive Machines.
		if !machineutils.IsMachineActive(machine) {
			continue
		}

		nodeTemplateChanged := copyMachineSetNodeTemplatesToMachines(machineSet, machine)
		// Only sync the machine that doesn't already have the latest nodeTemplate.
		if nodeTemplateChanged {
			_, err := UpdateMachineWithRetries(ctx, controlClient.Machines(machine.Namespace), machineLister, machine.Namespace, machine.Name,
				func(_ *v1alpha1.Machine) error {
					return nil
				})
			if err != nil {
				return fmt.Errorf("error in updating nodeTemplateSpec to machine %q: %v", machine.Name, err)
			}
			klog.V(2).Infof("Updated machine %s/%s of MachineSet %s/%s with latest nodeTemplate.", machine.Namespace, machine.Name, machineSet.Namespace, machineSet.Name)
		}
	}
	return nil
}

// syncMachinesClassKind updates all machines in the given machineList with the new classKind if required.
func (c *controller) syncMachinesClassKind(ctx context.Context, machineList []*v1alpha1.Machine, machineSet *v1alpha1.MachineSet) error {

	controlClient := c.controlMachineClient
	machineLister := c.machineLister

	for _, machine := range machineList {
		classKindChanged := copyMachineSetClassKindToMachines(machineSet, machine)
		// Only sync the machine that doesn't already have the matching classKind.
		if classKindChanged {
			_, err := UpdateMachineWithRetries(ctx, controlClient.Machines(machine.Namespace), machineLister, machine.Namespace, machine.Name,
				func(_ *v1alpha1.Machine) error {
					return nil
				})
			if err != nil {
				return fmt.Errorf("error in updating classKind to machine %q: %v", machine.Name, err)
			}
			klog.V(2).Infof("Updated Machine %s/%s of MachineSet %s/%s with latest classKind.", machine.Namespace, machine.Name, machineSet.Namespace, machineSet.Name)
		}
	}
	return nil
}

// copyMachineSetNodeTemplatesToMachines copies machineset's nodeTemplate to machine's nodeTemplate,
// and returns true if machine's nodeTemplate is changed.
// Note that apply and revision nodeTemplates are not copied.
func copyMachineSetNodeTemplatesToMachines(machineset *v1alpha1.MachineSet, machine *v1alpha1.Machine) bool {
	machineSetNodeTemplateCopy := machineset.Spec.Template.Spec.NodeTemplateSpec.DeepCopy()
	machineNodeTemplateCopy := machine.Spec.NodeTemplateSpec.DeepCopy()

	isNodeTemplateChanged := !(apiequality.Semantic.DeepEqual(machineSetNodeTemplateCopy, machineNodeTemplateCopy))

	if isNodeTemplateChanged {
		machine.Spec.NodeTemplateSpec = machineset.Spec.Template.Spec.NodeTemplateSpec
	}
	return isNodeTemplateChanged
}

// syncMachinesConfig updates all machines in the given machineList with the new config if required.
func (c *controller) syncMachinesConfig(ctx context.Context, machineList []*v1alpha1.Machine, machineSet *v1alpha1.MachineSet) error {

	controlClient := c.controlMachineClient
	machineLister := c.machineLister

	for _, machine := range machineList {
		// Ignore inactive Machines.
		if !machineutils.IsMachineActive(machine) {
			continue
		}

		configChanged := copyMachineSetConfigToMachines(machineSet, machine)
		// Only sync the machine that doesn't already have the latest config.
		if configChanged {
			_, err := UpdateMachineWithRetries(ctx, controlClient.Machines(machine.Namespace), machineLister, machine.Namespace, machine.Name,
				func(_ *v1alpha1.Machine) error {
					return nil
				})
			if err != nil {
				return fmt.Errorf("error in updating MachineConfig to machine %q: %v", machine.Name, err)
			}
			klog.V(2).Infof("Updated machine %s/%s of MachineSet %s/%s with latest config.", machine.Namespace, machine.Name, machineSet.Namespace, machineSet.Name)
		}
	}
	return nil
}

// copyMachineSetConfigToMachines copies machineset's config to machine's config,
// and returns true if machine's config is changed.
// Note that apply and revision config are not copied.
func copyMachineSetConfigToMachines(machineset *v1alpha1.MachineSet, machine *v1alpha1.Machine) bool {
	isConfigChanged := false

	machineSetConfigCopy := machineset.Spec.Template.Spec.MachineConfiguration.DeepCopy()
	machineConfigCopy := machine.Spec.MachineConfiguration.DeepCopy()

	isConfigChanged = !(apiequality.Semantic.DeepEqual(machineSetConfigCopy, machineConfigCopy))

	if isConfigChanged {
		machine.Spec.MachineConfiguration = machineset.Spec.Template.Spec.MachineConfiguration
	}
	return isConfigChanged
}

// copyMachineSetClassKindToMachines copies machineset's class.Kind to machine's class.Kind,
// and returns true if machine's class.Kind is changed.
func copyMachineSetClassKindToMachines(machineset *v1alpha1.MachineSet, machine *v1alpha1.Machine) bool {
	if machineset.Spec.Template.Spec.Class.Kind != machine.Spec.Class.Kind {
		machine.Spec.Class.Kind = machineset.Spec.Template.Spec.Class.Kind
		return true
	}

	return false
}

func logMachinesWithPriority1(machines []*v1alpha1.Machine) {
	for _, m := range machines {
		priority := m.Annotations[machineutils.MachinePriority]
		if priority == "1" {
			klog.V(3).Infof("Machine %q has %s annotation set to 1", m.Name, machineutils.MachinePriority)
		}
	}
}

func logMachinesToDelete(machines []*v1alpha1.Machine) {
	for _, m := range machines {
		klog.V(3).Infof("Machine %q needs to be deleted", m.Name)
	}
}

// triggerAutoPreservationOfFailedMachines annotates failed machines with the auto-preservation annotation
// to trigger preservation of the machines by the machine controller, up to the limit defined in the
// MachineSet's AutoPreserveFailedMachineMax field.
func (c *controller) triggerAutoPreservationOfFailedMachines(ctx context.Context, machines []*v1alpha1.Machine, machineSet *v1alpha1.MachineSet) {
	autoPreservationCapacityRemaining := machineSet.Spec.AutoPreserveFailedMachineMax - machineSet.Status.AutoPreserveFailedMachineCount
	if autoPreservationCapacityRemaining <= 0 {
		// no capacity remaining, nothing to do
		return
	}
	for _, m := range machines {
		if machineutils.IsMachineFailed(m) {
			// check if machine is annotated with preserve=false, if yes, do not consider for preservation
			if m.Annotations != nil && m.Annotations[machineutils.PreserveMachineAnnotationKey] == machineutils.PreserveMachineAnnotationValueFalse {
				continue
			}
			if autoPreservationCapacityRemaining > 0 {
				err := c.annotateMachineForAutoPreservation(ctx, m)
				if err != nil {
					klog.V(2).Infof("Error annotating machine %q for auto-preservation: %v", m.Name, err)
					// since annotateMachineForAutoPreservation uses retries internally, we can continue with other machines
					continue
				}
				autoPreservationCapacityRemaining = autoPreservationCapacityRemaining - 1
			}
		}
	}
}

// annotateMachineForAutoPreservation annotates the given machine with the auto-preservation annotation to trigger
// preservation of the machine by the machine controller.
func (dc *controller) annotateMachineForAutoPreservation(ctx context.Context, m *v1alpha1.Machine) error {
	_, err := UpdateMachineWithRetries(ctx, dc.controlMachineClient.Machines(m.Namespace), dc.machineLister, m.Namespace, m.Name, func(clone *v1alpha1.Machine) error {
		if clone.Annotations == nil {
			clone.Annotations = make(map[string]string)
		}
		clone.Annotations[machineutils.PreserveMachineAnnotationKey] = machineutils.PreserveMachineAnnotationValuePreservedByMCM
		return nil
	})
	if err != nil {
		return err
	}
	klog.V(2).Infof("Updated machine %q with %q=%q.", m.Name, machineutils.PreserveMachineAnnotationKey, machineutils.PreserveMachineAnnotationValuePreservedByMCM)
	return nil
}
