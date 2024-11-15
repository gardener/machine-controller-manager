// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package validation is used to validate all the machine CRD objects
package validation

import (
	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"math"
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

func canConvertIntOrStringToInt32(val *intstr.IntOrString, replicas int) bool {
	intVal, err := intstr.GetScaledValueFromIntOrPercent(val, replicas, true)
	if err != nil {
		return false
	}
	if intVal < math.MinInt32 || intVal > math.MaxInt32 {
		return false
	}
	return true
}

func validateUpdateStrategy(spec *machine.MachineDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.Strategy.Type != "RollingUpdate" && spec.Strategy.Type != "Recreate" {
		allErrs = append(allErrs, field.Required(fldPath.Child("strategy.type"), "Type can either be RollingUpdate or Recreate"))
	}
	if spec.Strategy.Type == "RollingUpdate" {
		if spec.Strategy.RollingUpdate == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("strategy.rollingUpdate"), "RollingUpdate parameter cannot be nil for rolling update strategy"))
		} else {
			if !canConvertIntOrStringToInt32(spec.Strategy.RollingUpdate.MaxUnavailable, int(spec.Replicas)) {
				allErrs = append(allErrs, field.Required(fldPath.Child("strategy.rollingUpdate.maxUnavailable"), "unable to convert maxUnavailable to int32"))
			}
			if !canConvertIntOrStringToInt32(spec.Strategy.RollingUpdate.MaxSurge, int(spec.Replicas)) {
				allErrs = append(allErrs, field.Required(fldPath.Child("strategy.rollingUpdate.maxSurge"), "unable to convert maxSurge to int32"))
			}
		}
	}
	return allErrs
}

func validateMachineDeploymentSpec(spec *machine.MachineDeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if spec.Replicas < 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("replicas"), "Replicas has to be a whole number"))
	}
	allErrs = append(allErrs, validateUpdateStrategy(spec, fldPath)...)
	for k, v := range spec.Selector.MatchLabels {
		if spec.Template.Labels[k] != v {
			allErrs = append(allErrs, field.Required(fldPath.Child("selector.matchLabels"), "is not matching with spec.template.metadata.labels"))
			break
		}
	}
	allErrs = append(allErrs, validateClassReference(&spec.Template.Spec.Class, field.NewPath("spec.template.spec.class"))...)
	return allErrs
}
