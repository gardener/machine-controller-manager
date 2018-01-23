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
	/*"strconv"
	"strings"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	*/
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/node-controller-manager/pkg/apis/machine"
)

// ValidateAzureMachineClass validates a AzureMachineClass and returns a list of errors.
func ValidateAzureMachineClass(AzureMachineClass *machine.AzureMachineClass) field.ErrorList {
	return internalValidateAzureMachineClass(AzureMachineClass)
}

func internalValidateAzureMachineClass(AzureMachineClass *machine.AzureMachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateAzureMachineClassSpec(&AzureMachineClass.Spec, field.NewPath("spec"))...)
	return allErrs
}

func validateAzureMachineClassSpec(spec *machine.AzureMachineClassSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}
