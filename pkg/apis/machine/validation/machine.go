// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package validation is used to validate all the machine CRD objects
package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateMachine and returns a list of errors.
func ValidateMachine(machine *machine.Machine) field.ErrorList {
	return internalValidateMachine(machine)
}

func internalValidateMachine(machine *machine.Machine) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateMachineSpec(&machine.Spec)...)
	return allErrs
}

func validateMachineSpec(spec *machine.MachineSpec) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateClassReference(&spec.Class, field.NewPath("spec.class"))...)
	return allErrs
}

func validateClassReference(classSpec *machine.ClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if classSpec.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "Kind is required"))
	}
	if classSpec.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "Name is required"))
	}

	return allErrs
}
