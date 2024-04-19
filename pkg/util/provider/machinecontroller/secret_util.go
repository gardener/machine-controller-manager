// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

// existsMachineClassForSecret checks for any machineClass
// referring to the passed secret object
// TODO: Check using finalizers on secrets
func (c *controller) existsMachineClassForSecret(name string) (bool, error) {
	MachineClasses, err := c.findMachineClassForSecret(name)
	if err != nil {
		return false, err
	}

	if len(MachineClasses) == 0 {
		return false, nil
	}

	return true, nil
}

// findMachineClassForSecret returns the set of
// MachineClasses referring to the passed secret
func (c *controller) findMachineClassForSecret(name string) ([]*v1alpha1.MachineClass, error) {
	machineClasses, err := c.machineClassLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var filtered []*v1alpha1.MachineClass
	for _, machineClass := range machineClasses {
		if (machineClass.SecretRef != nil && machineClass.SecretRef.Name == name) ||
			(machineClass.CredentialsSecretRef != nil && machineClass.CredentialsSecretRef.Name == name) {
			filtered = append(filtered, machineClass)
		}
	}
	return filtered, nil
}
