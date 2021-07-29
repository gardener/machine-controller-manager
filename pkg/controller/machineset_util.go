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

Modifications Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.
*/

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"

	labelsutil "github.com/gardener/machine-controller-manager/pkg/util/labels"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/retry"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	v1alpha1client "github.com/gardener/machine-controller-manager/pkg/client/clientset/versioned/typed/machine/v1alpha1"
	v1alpha1listers "github.com/gardener/machine-controller-manager/pkg/client/listers/machine/v1alpha1"
)

// TODO: use client library instead when it starts to support update retries
//       see https://github.com/kubernetes/kubernetes/issues/21479
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
		if !IsMachineActive(machine) {
			continue
		}

		nodeTemplateChanged := copyMachineSetNodeTemplatesToMachines(machineSet, machine)
		// Only sync the machine that doesn't already have the latest nodeTemplate.
		if nodeTemplateChanged {
			_, err := UpdateMachineWithRetries(ctx, controlClient.Machines(machine.Namespace), machineLister, machine.Namespace, machine.Name,
				func(machineToUpdate *v1alpha1.Machine) error {
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
				func(machineToUpdate *v1alpha1.Machine) error {
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
		if !IsMachineActive(machine) {
			continue
		}

		configChanged := copyMachineSetConfigToMachines(machineSet, machine)
		// Only sync the machine that doesn't already have the latest config.
		if configChanged {
			_, err := UpdateMachineWithRetries(ctx, controlClient.Machines(machine.Namespace), machineLister, machine.Namespace, machine.Name,
				func(machineToUpdate *v1alpha1.Machine) error {
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
