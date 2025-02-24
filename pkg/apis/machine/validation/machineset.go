// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package validation is used to validate all the machine CRD objects
package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateMachineSet and returns a list of errors.
func ValidateMachineSet(machineSet *machine.MachineSet) field.ErrorList {
	return internalValidateMachineSet(machineSet)
}

func internalValidateMachineSet(machineSet *machine.MachineSet) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateMachineSetSpec(&machineSet.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateMachineSetSpec(spec *machine.MachineSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Replicas < 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("replicas"), "Replicas has to be a whole number"))
	}

	for k, v := range spec.Selector.MatchLabels {
		if spec.Template.Labels[k] != v {
			allErrs = append(allErrs, field.Required(fldPath.Child("selector.matchLabels"), "is not matching with spec.template.metadata.labels"))
			break
		}
	}

	allErrs = append(allErrs, validateClassReference(&spec.Template.Spec.Class, field.NewPath("spec.template.spec.class"))...)
	return allErrs
}
