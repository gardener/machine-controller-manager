// /*
// Copyright (c) 2017 SAP SE or an SAP affiliate company. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

// // Package validation is used to validate all the machine CRD objects
package validation

import (
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine"
)

// validateName is the validation function for object names.
func validateMachineClassName(value string, prefix bool) []string {
	var errs []string
	if len(value) > nameMaxLength {
		errs = append(errs, utilvalidation.MaxLenError(nameMaxLength))
	}
	if !nameRegexp.MatchString(value) {
		errs = append(errs, utilvalidation.RegexError(nameFmt, "name-40d-0983-1b89"))
	}

	return errs
}

// ValidateMachineClass validates a MachineClass and returns a list of errors.
func ValidateMachineClass(MachineClass *machine.MachineClass) field.ErrorList {
	return internalValidateMachineClass(MachineClass)
}

func internalValidateMachineClass(MachineClass *machine.MachineClass) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&MachineClass.ObjectMeta, true, /*namespace*/
		validateName,
		field.NewPath("metadata"))...)

	allErrs = append(allErrs, validateMachineClassSpec(&MachineClass.ProviderSpec, field.NewPath("providerSpec"))...)
	return allErrs
}

func validateMachineClassSpec(spec *runtime.RawExtension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO: think of good validation here.
	return allErrs
}
