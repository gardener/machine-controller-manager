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

// ValidateMachineDeployment and returns a list of errors.
func ValidateMachineDeployment(machineDeployment *machine.MachineDeployment) field.ErrorList {
	return internalValidateMachineDeployment(machineDeployment)
}

func internalValidateMachineDeployment(machineDeployment *machine.MachineDeployment) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateMachineDeploymentSpec(&machineDeployment.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateMachineDeploymentSpec(spec *machine.MachineDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Replicas < 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("replicas"), "Replicas has to be a whole number"))
	}

	if spec.Strategy.Type != "RollingUpdate" && spec.Strategy.Type != "Recreate" {
		allErrs = append(allErrs, field.Required(fldPath.Child("strategy.type"), "Type can either be RollingUpdate or Recreate"))
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
