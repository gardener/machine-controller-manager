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
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

// ValidateOpenStackMachineClass validates a OpenStackMachineClass and returns a list of errors.
func ValidateOpenStackMachineClass(OpenStackMachineClass *machine.OpenStackMachineClass) field.ErrorList {
	return internalValidateOpenStackMachineClass(OpenStackMachineClass)
}

func internalValidateOpenStackMachineClass(OpenStackMachineClass *machine.OpenStackMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOpenStackMachineClassSpec(&OpenStackMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateOpenStackMachineClassSpec(spec *machine.OpenStackMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.ImageName {
		allErrs = append(allErrs, field.Required(fldPath.Child("imageName"), "ImageName is required"))
	}
	if "" == spec.Region {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "Region is required"))
	}
	if "" == spec.FlavorName {
		allErrs = append(allErrs, field.Required(fldPath.Child("flavorName"), "Flavor is required"))
	}
	if "" == spec.AvailabilityZone {
		allErrs = append(allErrs, field.Required(fldPath.Child("availabilityZone"), "AvailabilityZone Name is required"))
	}
	if "" == spec.KeyName {
		allErrs = append(allErrs, field.Required(fldPath.Child("keyName"), "KeyName is required"))
	}
	if "" == spec.NetworkID {
		allErrs = append(allErrs, field.Required(fldPath.Child("networkID"), "NetworkID is required"))
	}

	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)
	allErrs = append(allErrs, validateOSClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)

	return allErrs
}

func validateOSClassSpecTags(tags map[string]string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for key := range tags {
		if strings.Contains(key, "kubernetes.io-cluster-") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io-role-") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io-cluster-"), "Tag required of the form kubernetes.io-cluster-****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io-role-"), "Tag required of the form kubernetes.io-role-****"))
	}

	return allErrs
}
