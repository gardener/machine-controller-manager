// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package controller is used to provide the core functionalities of machine-controller-manager
package controller

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *controller) findMachinesForClass(kind, name string) ([]*v1alpha1.Machine, error) {
	machines, err := c.machineLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var filtered []*v1alpha1.Machine
	for _, machine := range machines {
		if machine.Spec.Class.Kind == kind && machine.Spec.Class.Name == name {
			filtered = append(filtered, machine)
		}
	}
	return filtered, nil
}
