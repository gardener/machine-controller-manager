/*
Copyright 2017 The Gardener Authors.

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
package validation

import (
	"code.sapcloud.io/kubernetes/node-controller-manager/pkg/apis/node"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateInstance and returns a list of errors.
func ValidateInstanceSet(instanceSet *node.InstanceSet) field.ErrorList {
	return internalValidateInstanceSet(instanceSet)
}

func internalValidateInstanceSet(instanceSet *node.InstanceSet) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateInstanceSetSpec(&instanceSet.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateInstanceSetSpec(spec *node.InstanceSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if 0 == spec.MinReadySeconds {
		allErrs = append(allErrs, field.Required(fldPath.Child("minReadySeconds"), "MinReadySeconds is required"))
	}

	allErrs = append(allErrs, validateClassReference(&spec.Template.Spec.Class, field.NewPath("spec.template.spec.class"))...)
	return allErrs
}

/*
func validateClassReference(classSpec *node.ClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if "" == classSpec.Kind {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "Kind is required"))
	}
	if "" == classSpec.Name {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "Name is required"))
	}

	return allErrs
}
*/