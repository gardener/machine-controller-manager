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
	"regexp"
	"strings"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

const metalNameFmt string = `[-a-z0-9]+`
const metalNameMaxLength int = 63

var metalNameRegexp = regexp.MustCompile("^" + metalNameFmt + "$")

// metalValidateName is the validation function for object names.
func metalValidateName(value string, prefix bool) []string {
	var errs []string
	if len(value) > nameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(metalNameMaxLength))
	}
	if !metalNameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(metalNameFmt, "name-40d-0983-1b89"))
	}

	return errs
}

// ValidateMetalMachineClass validates a MetalMachineClass and returns a list of errors.
func ValidateMetalMachineClass(MetalMachineClass *machine.MetalMachineClass) field.ErrorList {
	return internalValidateMetalMachineClass(MetalMachineClass)
}

func internalValidateMetalMachineClass(MetalMachineClass *machine.MetalMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&MetalMachineClass.ObjectMeta, true, /*namespace*/
		metalValidateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateMetalMachineClassSpec(&MetalMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateMetalMachineClassSpec(spec *machine.MetalMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if "" == spec.Image {
		allErrs = append(allErrs, field.Required(fldPath.Child("image"), "Image is required"))
	}
	if "" == spec.Size {
		allErrs = append(allErrs, field.Required(fldPath.Child("size"), "Size is required"))
	}
	if "" == spec.Project {
		allErrs = append(allErrs, field.Required(fldPath.Child("project"), "Project is required"))
	}
	if "" == spec.Network {
		allErrs = append(allErrs, field.Required(fldPath.Child("network"), "Network is required"))
	}
	if len(spec.Partition) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("partition"), "Partition is required"))
	}

	allErrs = append(allErrs, validateMetalClassSpecTags(spec.Tags, field.NewPath("spec.tags"))...)
	allErrs = append(allErrs, validateSecretRef(spec.SecretRef, field.NewPath("spec.secretRef"))...)

	return allErrs
}

func validateMetalClassSpecTags(tags []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	clusterName := ""
	nodeRole := ""

	for _, key := range tags {
		if strings.Contains(key, "kubernetes.io/cluster=") {
			clusterName = key
		} else if strings.Contains(key, "kubernetes.io/role=") {
			nodeRole = key
		}
	}

	if clusterName == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/cluster="), "Tag required of the form kubernetes.io/cluster=****"))
	}
	if nodeRole == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kubernetes.io/role="), "Tag required of the form kubernetes.io/role=****"))
	}

	return allErrs
}
