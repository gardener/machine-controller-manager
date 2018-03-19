/*
Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

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
	allErrs = append(allErrs, validateMachineSpec(&machine.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateMachineSpec(spec *machine.MachineSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateClassReference(&spec.Class, field.NewPath("spec.class"))...)
	return allErrs
}

func validateClassReference(classSpec *machine.ClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if "" == classSpec.Kind {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "Kind is required"))
	}
	if "" == classSpec.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "Name is required"))
	}

	return allErrs
}
